// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dnsconfig_test

import (
	"context"
	"time"

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

var _ = Describe("DNSServiceWatcher", func() {
	var (
		scheme              *runtime.Scheme
		kubeClient          client.Client
		dnsServiceWatcher   *dnsconfig.DNSServiceWatcher
		dnsServiceClusterIP string

		ctx       context.Context
		ctxCancel func()
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)

		kubeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		dnsServiceWatcher = &dnsconfig.DNSServiceWatcher{
			Client:      kubeClient,
			Namespace:   "capi-dns",
			ServiceName: "dns-server",

			PollingInterval: 2 * time.Millisecond,
		}

		dnsServiceClusterIP = "1.2.3.4"
		ctx, ctxCancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	})

	It("returns the ClusterIP for the DNS service", func() {
		startupDNSService(kubeClient, dnsServiceClusterIP)

		clusterIP, err := dnsServiceWatcher.GetDNSServiceClusterIP(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(clusterIP).To(Equal(dnsServiceClusterIP))
	})

	Context("when the CAPI DNS service is not found", func() {
		It("returns an error", func() {
			clusterIP, err := dnsServiceWatcher.GetDNSServiceClusterIP(ctx)
			Expect(err).To(MatchError(`Timed out obtaining ClusterIP from service "capi-dns/dns-server": services "dns-server" not found`))
			Expect(clusterIP).To(BeEmpty())
		})
	})

	Context("when the CAPI DNS service does not acquire a ClusterIP before the context is done", func() {
		It("does not modify the system Corefile", func() {
			startupDNSService(kubeClient, dnsServiceClusterIP)

			ctxCancel()

			clusterIP, err := dnsServiceWatcher.GetDNSServiceClusterIP(ctx)
			Expect(err).To(MatchError(`Timed out obtaining ClusterIP from service "capi-dns/dns-server": service "capi-dns/dns-server" does not have a ClusterIP`))
			Expect(clusterIP).To(BeEmpty())
		})
	})
})

func startupDNSService(kubeClient client.Client, desiredClusterIP string) {
	dnsServerService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dns-server",
			Namespace: "capi-dns",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
		},
	}
	err := kubeClient.Create(context.Background(), &dnsServerService)
	Expect(err).NotTo(HaveOccurred())

	time.AfterFunc(10*time.Millisecond, func() {
		dnsServerService.Spec.ClusterIP = desiredClusterIP

		err = kubeClient.Update(context.Background(), &dnsServerService)
		Expect(err).NotTo(HaveOccurred())
	})
}
