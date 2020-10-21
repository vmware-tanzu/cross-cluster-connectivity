// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package connectivity_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	kubectlTimeout     = 1 * time.Minute
	kubectlInterval    = 5 * time.Second
	curlConnectTimeoutInSeconds = "5"
)

func TestConnectivity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Connectivity Suite")
}

func kubectlWithConfig(kubeConfigPath string, args ...string) ([]byte, error) {
	if len(kubeConfigPath) == 0 {
		return nil, errors.New("kubeconfig path cannot be empty")
	}
	argsWithKubeConfig := append([]string{"--kubeconfig", kubeConfigPath}, args...)

	cmd := exec.Command("kubectl", argsWithKubeConfig...)
	cmd.Stderr = GinkgoWriter

	fmt.Fprintf(GinkgoWriter, " + kubectl %s\n", strings.Join(args, " "))
	output, err := cmd.Output()
	return output, err
}

var workloadClusterKubeConfig = os.Getenv("WORKLOAD_CLUSTER_KUBECONFIG")
var sharedServiceClusterKubeConfig = os.Getenv("SHARED_SERVICE_CLUSTER_KUBECONFIG")

var _ = BeforeSuite(func() {
	workloadClusterKubeConfig := os.Getenv("WORKLOAD_CLUSTER_KUBECONFIG")
	sharedServiceClusterKubeConfig := os.Getenv("SHARED_SERVICE_CLUSTER_KUBECONFIG")

	if len(workloadClusterKubeConfig) == 0 {
		Fail("WORKLOAD_CLUSTER_KUBECONFIG not set")
	}
	if len(sharedServiceClusterKubeConfig) == 0 {
		Fail("SHARED_SERVICE_CLUSTER_KUBECONFIG not set")
	}
})
