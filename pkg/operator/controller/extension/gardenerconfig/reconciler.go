// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/operator/apis/config"
)

const (
	requeueUnhealthyVirtualKubeApiserver = 10 * time.Second
)

// Reconciler reconciles Gardens.
type Reconciler struct {
	RuntimeClientSet kubernetes.Interface
	RuntimeVersion   *semver.Version
	Config           config.OperatorConfiguration
	Clock            clock.Clock
	Recorder         record.EventRecorder
	Identity         *gardencorev1beta1.Gardener
	GardenClientMap  clientmap.ClientMap
	GardenNamespace  string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	gardenList := &operatorv1alpha1.GardenList{}
	// We limit one result because we expect only a single Garden object to be there.
	if err := r.RuntimeClientSet.Client().List(ctx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden object: %w", err)
	}
	if len(gardenList.Items) == 0 {
		return reconcile.Result{}, fmt.Errorf("error: garden object not found")
	}

	garden := &gardenList.Items[0]
	virtualKubeAPIServerCond := v1beta1helper.GetOrInitConditionWithClock(r.Clock, garden.Status.Conditions, operatorv1alpha1.VirtualGardenAPIServerAvailable)
	if virtualKubeAPIServerCond.Status != gardencorev1beta1.ConditionTrue {
		log.Info("virtual garden is not ready yet. Requeue...")
		return reconcile.Result{}, &reconciler.RequeueAfterError{
			RequeueAfter: requeueUnhealthyVirtualKubeApiserver,
		}
	}

	gardenClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden client object: %w", err)
	}

	if extension.DeletionTimestamp != nil {
		panic("delete me")
		return reconcile.Result{}, nil
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.ExtensionGardenConfig.SyncPeriod.Duration}, r.reconcile(ctx, gardenClientSet.Client(), extension)
}

func (r *Reconciler) reconcile(ctx context.Context, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	ctrlReg := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlReg, func() error {
		ctrlReg.Annotations = extension.Spec.Deployment.Extension.Annotations
		ctrlReg.Spec = gardencorev1beta1.ControllerRegistrationSpec{
			Resources: extension.Spec.Resources,
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				Policy: extension.Spec.Deployment.Extension.Policy,
				// TODO
				SeedSelector: nil,
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{
					{
						Name: extension.Name,
					},
				},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create or update ControllerRegistration: %w", err)
	}

	ctrlDeploy := &gardencorev1beta1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlDeploy, func() error {
		ctrlDeploy.Annotations = extension.Spec.Deployment.Extension.Annotations
		ctrlDeploy.Type = "helm"

		var helmDeployment struct {
			// chart is a Helm chart tarball.
			Chart []byte `json:"chart,omitempty"`
			// Values is a map of values for the given chart.
			Values map[string]interface{} `json:"values,omitempty"`
		}

		// TODO: nil checks
		helm := extension.Spec.Deployment.Extension.Helm
		if rawChart := helm.RawChart; rawChart != nil {
			helmDeployment.Chart = rawChart
		}
		if values := helm.Values; values != nil {
			if err := json.Unmarshal(values.Raw, helm.Values); err != nil {
				return err
			}
		}

		rawHelm, err := json.Marshal(helm)
		if err != nil {
			return err
		}

		ctrlDeploy.ProviderConfig = runtime.RawExtension{
			Raw: rawHelm,
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create or update ControllerInstallation: %w", err)
	}

	return nil
}
