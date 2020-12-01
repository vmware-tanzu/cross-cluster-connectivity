// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package connectivity_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EndpointSlice DNS", func() {
	AfterEach(func() {
		kubectlWithConfig(clusterOneKubeConfig,
			"delete", "-f", filepath.Join("fixtures", "gateway-endpoint-slice.yaml"))
	})

	It("journeys", func() {
		By("create an EndpointSlice with an annotation for the wildcard DNS name")
		_, err := kubectlWithConfig(clusterOneKubeConfig,
			"apply", "-f", filepath.Join("fixtures", "gateway-endpoint-slice.yaml"))
		Expect(err).NotTo(HaveOccurred())

		By("validating that the wildcard DNS name resolves")
		Eventually(func() string {
			output, _ := kubectlWithConfig(clusterOneKubeConfig,
				"run", "host-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--command", "--",
				"nslookup", "bar.gateway.strawberry.foo-ns.clusters.xcc.test")
			return string(output)
		}, kubectlTimeout, kubectlInterval).Should(And(ContainSubstring("10.4.5.6"), ContainSubstring("10.4.5.7")))

		By("deleting the EndpointSlice")
		_, err = kubectlWithConfig(clusterOneKubeConfig,
			"delete", "-f", filepath.Join("fixtures", "gateway-endpoint-slice.yaml"))
		Expect(err).NotTo(HaveOccurred())

		By("validating that the wildcard DNS name no longer resolves")
		Eventually(func() string {
			output, _ := kubectlWithConfig(clusterOneKubeConfig,
				"run", "host-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--command", "--",
				"nslookup", "bar.gateway.strawberry.foo-ns.clusters.xcc.test")
			return string(output)
		}, kubectlTimeout, kubectlInterval).Should(ContainSubstring("server can't find bar.gateway.strawberry.foo-ns.clusters.xcc.test: NXDOMAIN"))
	})
})
