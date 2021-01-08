// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterapidns_test

import (
	"crypto/rand"
	"crypto/rsa"
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
	. "github.com/onsi/gomega"
)

var _ = Describe("ClusterAPI DNS Test", func() {
	var certsDir string
	var fqdn = "nginx.gateway.cluster-a.dev-team.clusters.xcc.test"

	BeforeEach(func() {
		By("deploy nginx on cluster-a")

		var err error
		certsDir, err = ioutil.TempDir("", "cluster-api-dns-test")
		Expect(err).NotTo(HaveOccurred())

		By("generating a certificate for the fqdn")
		cert, key, err := generateCert(fqdn)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "cert.pem"), cert, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(certsDir, "key.pem"), key, 0755)
		Expect(err).NotTo(HaveOccurred())

		By("deploying nginx to cluster-a")
		deployNginx(clusterAKubeConfig, certsDir, "cluster-a")
	})

	AfterEach(func() {
		_, _ = kubectlWithConfig(clusterAKubeConfig, "delete", "namespace", "nginx-test")

		Expect(os.RemoveAll(certsDir)).To(Succeed())
	})

	It("journeys", func() {
		By("create a GatewayDNS on management cluster referencing Contour in dev-team namespace")
		_, err := kubectlWithConfig(managementKubeConfig,
			"apply", "-f", filepath.Join("fixtures", "dev-team-gateway-dns.yaml"))
		Expect(err).NotTo(HaveOccurred())

		By("validating that the wildcard DNS name resolves on cluster-a")
		Eventually(func() string {
			return curlOnCluster(clusterAKubeConfig, fqdn)
		}, kubectlTimeout, kubectlInterval).Should(And(
			ContainSubstring("HTTP/2 200"),
			ContainSubstring("x-cluster: cluster-a"),
		))

		By("validating that the wildcard DNS name resolves on cluster-b")
		Eventually(func() string {
			return curlOnCluster(clusterBKubeConfig, fqdn)
		}, kubectlTimeout, kubectlInterval).Should(And(
			ContainSubstring("HTTP/2 200"),
			ContainSubstring("x-cluster: cluster-a"),
		))

		By("deleting the GatewayDNSRecord")
		_, err = kubectlWithConfig(managementKubeConfig,
			"delete", "-f", filepath.Join("fixtures", "dev-team-gateway-dns.yaml"))
		Expect(err).NotTo(HaveOccurred())

		By("validating that the wildcard DNS name no longer resolves on cluster-a")
		Eventually(func() string {
			return curlOnCluster(clusterAKubeConfig, fqdn)
		}, kubectlTimeout, kubectlInterval).Should(ContainSubstring("Could not resolve host"))

		By("validating that the wildcard DNS name no longer resolves on cluster-b")
		Eventually(func() string {
			return curlOnCluster(clusterBKubeConfig, fqdn)
		}, kubectlTimeout, kubectlInterval).Should(ContainSubstring("Could not resolve host"))
	})
})

func curlOnCluster(kubeConfig, fqdn string) string {
	output, _ := kubectlWithConfig(kubeConfig,
		"run", "nginx-test", "-i", "--rm", "--image=curlimages/curl", "--restart=Never", "--",
		"curl", "-v", "-k", "--connect-timeout", curlConnectTimeoutInSeconds, fmt.Sprintf("https://%s", fqdn))
	return string(output)
}

func deployNginx(kubeconfig, certsDir, clusterHeaderValue string) {
	_, err := kubectlWithConfig(kubeconfig, "create", "namespace", "nginx-test")
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

	_, err = kubectlWithConfig(kubeconfig, "apply", "-f", nginxConfFile.Name())
	Expect(err).NotTo(HaveOccurred())

	Expect(os.RemoveAll(nginxConfFile.Name())).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"apply", "-f", filepath.Join("..", "..", "manifests", "example", "nginx", "nginx.yaml"))
	Expect(err).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"apply", "-f", filepath.Join("..", "..", "manifests", "example", "nginx", "exported_http_proxy.yaml"))
	Expect(err).NotTo(HaveOccurred())

	nginxDeploymentPatch, err := ioutil.ReadFile(filepath.Join("fixtures", "nginx-deployment-patch.yaml"))
	Expect(err).NotTo(HaveOccurred())

	_, err = kubectlWithConfig(kubeconfig,
		"-n", "nginx-test", "patch", "deployment", "nginx", "--patch", string(nginxDeploymentPatch))
	Expect(err).NotTo(HaveOccurred())
}

func generateCert(fqdn string) (cert []byte, key []byte, err error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	// Generate the private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"ClusterAPIDNS Test"},
		},
		DNSNames:              []string{fqdn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
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
