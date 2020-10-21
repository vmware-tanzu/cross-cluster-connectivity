// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package connectivity_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"time"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exported Service", func() {
	var certsDir string

	BeforeEach(func() {
		var err error
		certsDir, err = ioutil.TempDir("", "connectivity-test")
		Expect(err).NotTo(HaveOccurred())

		cert, key, err := generateCert("nginx.xcc.test")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "cert.pem"), cert, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "key.pem"), key, 0755)
		Expect(err).NotTo(HaveOccurred())

		_, err = kubectlWithConfig(sharedServiceClusterKubeConfig,
			"create", "namespace", "nginx-test")
		Expect(err).NotTo(HaveOccurred())

		_, err = kubectlWithConfig(sharedServiceClusterKubeConfig,
			"create", "secret", "-n", "nginx-test", "tls", "nginx-tls-secret",
			"--cert", filepath.Join(certsDir, "cert.pem"),
			"--key", filepath.Join(certsDir, "key.pem"),
		)
		Expect(err).NotTo(HaveOccurred())

		// Deploy nginx yaml to shared-service cluster
		_, err = kubectlWithConfig(sharedServiceClusterKubeConfig,
			"apply", "-f", filepath.Join("..", "..", "manifests", "example", "nginx.yaml"))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(certsDir)).To(Succeed())

		_, err := kubectlWithConfig(sharedServiceClusterKubeConfig,
			"delete", "namespace", "nginx-test")
		Expect(err).NotTo(HaveOccurred())

		By("ensuring cross-cluster-connectivity managed resources are cleaned up")
		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(sharedServiceClusterKubeConfig,
				"get", "servicerecord", "-n", connectivityv1alpha1.ConnectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring("nginx.xcc.test"))

		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(workloadClusterKubeConfig,
				"get", "servicerecord", "-n", connectivityv1alpha1.ConnectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring("nginx.xcc.test"))

		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(workloadClusterKubeConfig,
				"get", "service", "-n", connectivityv1alpha1.ConnectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring("nginx-xcc-test"))
	})

	It("journeys", func() {
		By("validating it doesn't discover the shared service on the workload cluster")
		Consistently(func() (string, error) {
			output, err := kubectlWithConfig(workloadClusterKubeConfig,
				"get", "svc", "-n", connectivityv1alpha1.ConnectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring("nginx-xcc-test"))

		By("declaring intent to export the shared service to the workload cluster")
		_, err := kubectlWithConfig(sharedServiceClusterKubeConfig,
			"apply", "-f", filepath.Join("..", "..", "manifests", "example", "exported_http_proxy.yaml"))
		Expect(err).NotTo(HaveOccurred())

		By("validating it can connect to the shared service on the workload cluster")
		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(workloadClusterKubeConfig,
				"run", "nginx-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--",
				"curl", "-v", "-k", "--connect-timeout", curlConnectTimeoutInSeconds, "https://nginx.xcc.test")
			return string(output), err
		}, kubectlTimeout, kubectlInterval).Should(And(
			ContainSubstring("HTTP/2 200"),
			ContainSubstring("Thank you for using nginx."),
		))

		By("remove intent to export the shared service to the workload cluster")
		_, err = kubectlWithConfig(sharedServiceClusterKubeConfig,
			"-n", "nginx-test", "label", "httpproxy", "nginx-service-ingress", "connectivity.tanzu.vmware.com/export-")
		Expect(err).NotTo(HaveOccurred())

		By("validating it cannot connect to the shared service on the workload cluster")
		Eventually(func() string {
			output, _ := kubectlWithConfig(workloadClusterKubeConfig,
				"run", "nginx-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--",
				"curl", "-s", "-S", "--stderr", "-", "-k", "--connect-timeout", curlConnectTimeoutInSeconds, "https://nginx.xcc.test")
			return string(output)
		}, kubectlTimeout, kubectlInterval).Should(ContainSubstring("Could not resolve host"))
	})
})

func generateCert(fqdn string) (cert []byte, key []byte, err error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	// Generate the private key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Connectivity Test"},
		},
		DNSNames:              []string{fqdn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		return nil, nil, err
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	return
}
