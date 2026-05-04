// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logger_test

import (
	"bytes"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("WithConsoleErrorSink", func() {
	var (
		originalStdout *os.File
		stdoutReader   *os.File
		stdoutWriter   *os.File
	)

	BeforeEach(func() {
		// Capture stdout
		originalStdout = os.Stdout
		var err error
		stdoutReader, stdoutWriter, err = os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		os.Stdout = stdoutWriter
	})

	AfterEach(func() {
		// Restore stdout
		os.Stdout = originalStdout
		if stdoutWriter != nil {
			stdoutWriter.Close()
		}
		if stdoutReader != nil {
			stdoutReader.Close()
		}
	})

	It("should write error-level logs to stdout", func() {
		logger, err := NewZapLogger(InfoLevel, FormatText, WithConsoleErrorSink())
		Expect(err).NotTo(HaveOccurred())

		// Log an error
		logger.Error(nil, "test error message")

		// Close writer and read captured output
		stdoutWriter.Close()
		var buf bytes.Buffer
		_, err = buf.ReadFrom(stdoutReader)
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		Expect(output).To(ContainSubstring("ERROR: test error message"))
	})

	It("should not write info-level logs to stdout", func() {
		logger, err := NewZapLogger(InfoLevel, FormatText, WithConsoleErrorSink())
		Expect(err).NotTo(HaveOccurred())

		// Log info
		logger.Info("test info message")

		// Close writer and read captured output
		stdoutWriter.Close()
		var buf bytes.Buffer
		_, err = buf.ReadFrom(stdoutReader)
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		Expect(output).To(BeEmpty(), "info logs should not go to stdout")
	})
})
