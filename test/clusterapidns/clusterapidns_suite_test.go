// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterapidns_test

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
	//TODO: Choose good timeout/interval values
	kubectlTimeout              = 60 * time.Second
	kubectlInterval             = 5 * time.Second
	curlConnectTimeoutInSeconds = "3"
)

func TestClusterapidns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster API DNS Suite")
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

var managementKubeConfig = os.Getenv("MANAGEMENT_KUBECONFIG")
var clusterAKubeConfig = os.Getenv("CLUSTER_A_KUBECONFIG")
var clusterBKubeConfig = os.Getenv("CLUSTER_B_KUBECONFIG")

var _ = BeforeSuite(func() {
	if len(managementKubeConfig) == 0 {
		Fail("MANAGEMENT_KUBECONFIG not set")
	}
	if len(clusterAKubeConfig) == 0 {
		Fail("CLUSTER_A_KUBECONFIG not set")
	}
	if len(clusterBKubeConfig) == 0 {
		Fail("CLUSTER_B_KUBECONFIG not set")
	}
})
