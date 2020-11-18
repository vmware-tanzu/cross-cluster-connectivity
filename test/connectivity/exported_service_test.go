// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package connectivity_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var connectivityNamespace = "cross-cluster-connectivity"

var _ = Describe("Exported Service", func() {
	var certsDir string
	var clusterToCleanupKubeConfig string

	BeforeEach(func() {
		var err error
		certsDir, err = ioutil.TempDir("", "connectivity-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(certsDir)).To(Succeed())

		kubectlWithConfig(clusterToCleanupKubeConfig,
			"delete", "namespace", "nginx-test")
	})

	DescribeTable("journeys", func(clientClusterKubeConfig, servicesClusterKubeConfig, servicesClusterID, fqdn string) {
		clusterToCleanupKubeConfig = servicesClusterKubeConfig

		By("generating a certificate for the fqdn")
		cert, key, err := generateCert(fqdn)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "cert.pem"), cert, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "key.pem"), key, 0755)
		Expect(err).NotTo(HaveOccurred())

		deployNginx(servicesClusterKubeConfig, certsDir, servicesClusterID)

		By("validating it doesn't discover the published service on the client cluster")
		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(clientClusterKubeConfig,
				"get", "svc", "-n", connectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring(fqdn))

		By("declaring intent to export the service to the client cluster")
		_, err = kubectlWithConfig(servicesClusterKubeConfig,
			"apply", "-f", generateExportedHTTPProxyFileWithFQDN(fqdn))
		Expect(err).NotTo(HaveOccurred())

		By("validating it can connect to the published service on the client cluster")
		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(clientClusterKubeConfig,
				"run", "nginx-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--",
				"curl", "-v", "-k", "--connect-timeout", curlConnectTimeoutInSeconds, fmt.Sprintf("https://%s", fqdn))
			return string(output), err
		}, kubectlTimeout, kubectlInterval).Should(And(
			ContainSubstring("HTTP/2 200"),
			ContainSubstring(fmt.Sprintf("x-cluster: %s", servicesClusterID)),
		))

		By("remove intent to export the published service to the client cluster")
		_, err = kubectlWithConfig(servicesClusterKubeConfig,
			"-n", "nginx-test", "label", "httpproxy", "nginx-service-ingress", "connectivity.tanzu.vmware.com/export-")
		Expect(err).NotTo(HaveOccurred())

		By("validating it cannot connect to the published service on the client cluster")
		Eventually(func() string {
			output, _ := kubectlWithConfig(clientClusterKubeConfig,
				"run", "nginx-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--",
				"curl", "-s", "-S", "--stderr", "-", "-k", "--connect-timeout", curlConnectTimeoutInSeconds, fmt.Sprintf("https://%s", fqdn))
			return string(output)
		}, kubectlTimeout, kubectlInterval).Should(ContainSubstring("Could not resolve host"))

		_, err = kubectlWithConfig(servicesClusterKubeConfig,
			"delete", "namespace", "nginx-test")
		Expect(err).NotTo(HaveOccurred())

		By("ensuring cross-cluster-connectivity managed resources are cleaned up")
		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(servicesClusterKubeConfig,
				"get", "servicerecord", "-n", connectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring(fqdn))

		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(clientClusterKubeConfig,
				"get", "servicerecord", "-n", connectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring(fqdn))

		Eventually(func() (string, error) {
			output, err := kubectlWithConfig(clientClusterKubeConfig,
				"get", "service", "-n", connectivityNamespace)
			return string(output), err
		}, kubectlTimeout, kubectlInterval).ShouldNot(ContainSubstring(fqdn))
	},
		Entry("tests connectivity from cluster one to cluster two", clusterOneKubeConfig, clusterTwoKubeConfig, "cluster-two", "nginx-cluster-two.xcc.test"),
		Entry("tests connectivity from cluster two to cluster one", clusterTwoKubeConfig, clusterOneKubeConfig, "cluster-one", "nginx-cluster-one.xcc.test"),
	)
})

func deployNginx(kubeconfig, certsDir, clusterHeaderValue string) {
	_, err := kubectlWithConfig(kubeconfig,
		"create", "namespace", "nginx-test")
	Expect(err).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"create", "secret", "-n", "nginx-test", "tls", "nginx-tls-secret",
		"--cert", filepath.Join(certsDir, "cert.pem"),
		"--key", filepath.Join(certsDir, "key.pem"),
	)
	Expect(err).NotTo(HaveOccurred())

	nginxConfTemplate, err := ioutil.ReadFile(filepath.Join("fixtures", "nginx-conf.yaml"))
	Expect(err).NotTo(HaveOccurred())

	nginxConf := strings.Replace(string(nginxConfTemplate), "REPLACE_CLUSTER_HEADER_VALUE", clusterHeaderValue, 1)

	nginxConfFile, err := ioutil.TempFile("", "nginx-conf")
	Expect(err).NotTo(HaveOccurred())

	_, err = nginxConfFile.Write([]byte(nginxConf))
	Expect(err).NotTo(HaveOccurred())
	Expect(nginxConfFile.Close()).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"apply", "-f", nginxConfFile.Name())
	Expect(err).NotTo(HaveOccurred())

	Expect(os.RemoveAll(nginxConfFile.Name())).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"apply", "-f", filepath.Join("..", "..", "manifests", "example", "nginx", "nginx.yaml"))
	Expect(err).NotTo(HaveOccurred())

	nginxDeploymentPatch, err := ioutil.ReadFile(filepath.Join("fixtures", "nginx-deployment-patch.yaml"))
	Expect(err).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"-n", "nginx-test", "patch", "deployment", "nginx", "--patch", string(nginxDeploymentPatch))
	Expect(err).NotTo(HaveOccurred())
}

func generateExportedHTTPProxyFileWithFQDN(fqdn string) string {
	exportedHTTPProxyTemplate, err := ioutil.ReadFile(filepath.Join("..", "..", "manifests", "example", "nginx", "exported_http_proxy.yaml"))
	Expect(err).NotTo(HaveOccurred())

	exportedHTTPProxy := strings.Replace(string(exportedHTTPProxyTemplate), "fqdn: nginx.xcc.test", fmt.Sprintf("fqdn: %s", fqdn), 1)

	exportedHTTPProxyFile, err := ioutil.TempFile("", "exported_http_proxy")
	Expect(err).NotTo(HaveOccurred())

	_, err = exportedHTTPProxyFile.Write([]byte(exportedHTTPProxy))
	Expect(err).NotTo(HaveOccurred())
	Expect(exportedHTTPProxyFile.Close()).NotTo(HaveOccurred())

	return exportedHTTPProxyFile.Name()
}

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
