// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerconfig

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/operator/apis/config"
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
	gardenClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden client object: %w", err)
	}

	if extension.DeletionTimestamp != nil {
		panic("delete me")
		return reconcile.Result{}, nil
	}

	panic("reconcile me")
	// } else if result.Requeue {
	// 	return result, nil
	// }
	// return reconcile.Result{RequeueAfter: r.Config.Controllers.Garden.SyncPeriod.Duration}, r.updateStatusOperationSuccess(ctx, garden, operationType)
	return reconcile.Result{}, nil
}
