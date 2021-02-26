// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dnsconfig_test

import (
	"context"
	"strings"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/dnsconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CorefilePatcher", func() {
	var (
		scheme *runtime.Scheme

		kubeClient client.Client

		patcher          *dnsconfig.CorefilePatcher
		corednsConfigMap corev1.ConfigMap

		updatedCorednsConfigMap corev1.ConfigMap

		forwardingIP string
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)

		kubeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		log := ctrl.Log.WithName("dnsconfig").WithName("CorefilePatcher")

		patcher = &dnsconfig.CorefilePatcher{
			Client:        kubeClient,
			Log:           log,
			DomainSuffix:  "xcc.test",
			Namespace:     "kube-system",
			ConfigMapName: "coredns",
		}

		corednsConfigMap = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"Corefile": strings.Join([]string{
					".:53 {",
					"    original_zone_content",
					"}",
				}, "\n"),
			},
		}

		forwardingIP = "1.2.3.4"
	})

	It("appends a server block for the provided domain suffix to forward to the dns server service cluster ip", func() {
		err := kubeClient.Create(context.Background(), &corednsConfigMap)
		Expect(err).NotTo(HaveOccurred())

		err = patcher.AppendStubDomainBlock(forwardingIP)
		Expect(err).NotTo(HaveOccurred())

		err = kubeClient.Get(context.Background(), client.ObjectKey{
			Name:      "coredns",
			Namespace: "kube-system",
		}, &updatedCorednsConfigMap)
		Expect(err).NotTo(HaveOccurred())

		Expect(updatedCorednsConfigMap.Data["Corefile"]).To(Equal(strings.Join([]string{
			".:53 {",
			"    original_zone_content",
			"}",
			"### BEGIN CROSS CLUSTER CONNECTIVITY",
			"xcc.test {",
			"    forward . 1.2.3.4",
			"    reload",
			"}",
			"### END CROSS CLUSTER CONNECTIVITY",
			"",
		}, "\n")))
	})

	Context("when the system corefile configmap has already been updated", func() {
		var originalCorefile string
		BeforeEach(func() {
			originalCorefile = strings.Join([]string{
				".:53 {",
				"    original_zone_content",
				"}",
				"### BEGIN CROSS CLUSTER CONNECTIVITY",
				"xcc.test {",
				"    forward . 1.2.3.4",
				"    reload",
				"}",
				"### END CROSS CLUSTER CONNECTIVITY",
				"",
			}, "\n")

			corednsConfigMap.Data["Corefile"] = originalCorefile
			err := kubeClient.Create(context.Background(), &corednsConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not append a second server block", func() {
			err := patcher.AppendStubDomainBlock(forwardingIP)
			Expect(err).NotTo(HaveOccurred())

			err = kubeClient.Get(context.Background(), client.ObjectKey{
				Name:      "coredns",
				Namespace: "kube-system",
			}, &updatedCorednsConfigMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedCorednsConfigMap.Data["Corefile"]).To(Equal(originalCorefile))
		})
	})

	Context("when the system corefile configmap has a different xcc-dns domain", func() {
		BeforeEach(func() {
			corednsConfigMap.Data["Corefile"] = strings.Join([]string{
				".:53 {",
				"    original_zone_content",
				"}",
				"### BEGIN CROSS CLUSTER CONNECTIVITY",
				"xcc-different.test {",
				"    forward . 1.2.3.4",
				"    reload",
				"}",
				"### END CROSS CLUSTER CONNECTIVITY",
				"",
				"other-zone.foobar {",
				"    forward . 1.2.3.5",
				"    reload",
				"}",
			}, "\n")
			err := kubeClient.Create(context.Background(), &corednsConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the configmap", func() {
			err := patcher.AppendStubDomainBlock(forwardingIP)
			Expect(err).NotTo(HaveOccurred())

			err = kubeClient.Get(context.Background(), client.ObjectKey{
				Name:      "coredns",
				Namespace: "kube-system",
			}, &updatedCorednsConfigMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedCorednsConfigMap.Data["Corefile"]).To(Equal(strings.Join([]string{
				".:53 {",
				"    original_zone_content",
				"}",
				"",
				"other-zone.foobar {",
				"    forward . 1.2.3.5",
				"    reload",
				"}",
				"### BEGIN CROSS CLUSTER CONNECTIVITY",
				"xcc.test {",
				"    forward . 1.2.3.4",
				"    reload",
				"}",
				"### END CROSS CLUSTER CONNECTIVITY",
				"",
			}, "\n")))
		})
	})

	Context("when the system corefile configmap has a different xcc-dns IP", func() {
		BeforeEach(func() {
			corednsConfigMap.Data["Corefile"] = strings.Join([]string{
				".:53 {",
				"    original_zone_content",
				"}",
				"",
				"### BEGIN CROSS CLUSTER CONNECTIVITY",
				"xcc.test {",
				"    forward . 42.42.42.42",
				"    reload",
				"}",
				"### END CROSS CLUSTER CONNECTIVITY",
				"",
			}, "\n")

			err := kubeClient.Create(context.Background(), &corednsConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the configmap", func() {
			err := patcher.AppendStubDomainBlock(forwardingIP)
			Expect(err).NotTo(HaveOccurred())

			err = kubeClient.Get(context.Background(), client.ObjectKey{
				Name:      "coredns",
				Namespace: "kube-system",
			}, &updatedCorednsConfigMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedCorednsConfigMap.Data["Corefile"]).To(Equal(strings.Join([]string{
				".:53 {",
				"    original_zone_content",
				"}",
				"### BEGIN CROSS CLUSTER CONNECTIVITY",
				"xcc.test {",
				"    forward . 1.2.3.4",
				"    reload",
				"}",
				"### END CROSS CLUSTER CONNECTIVITY",
				"",
			}, "\n")))
		})
	})
})
