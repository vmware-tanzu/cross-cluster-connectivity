// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryserver"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
)

func Test_RegistryPeering(t *testing.T) {
	testcases := []struct {
		name                string
		remoteRegistry      *connectivityv1alpha1.RemoteRegistry
		exportServiceRecord *connectivityv1alpha1.ServiceRecord
		importServiceRecord *connectivityv1alpha1.ServiceRecord
	}{
		{
			name: "standard exported ServiceRecord",
			remoteRegistry: &connectivityv1alpha1.RemoteRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-services-cls",
					Namespace: "cross-cluster-connectivity",
				},
				Spec: connectivityv1alpha1.RemoteRegistrySpec{
					Address: "127.0.0.1:8000",
					TLSConfig: connectivityv1alpha1.RegistryTLSConfig{
						ServerName: "foobar.some.domain",
					},
				},
			},
			exportServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
			importServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain-6068d556",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ImportedLabel:                   "",
						connectivityv1alpha1.ConnectivityRemoteRegistryLabel: "shared-services-cls",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
		},
		{
			name: "exported ServiceRecord with connectivity VIP annotations",
			remoteRegistry: &connectivityv1alpha1.RemoteRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-services-cls",
					Namespace: "cross-cluster-connectivity",
				},
				Spec: connectivityv1alpha1.RemoteRegistrySpec{
					Address: "127.0.0.1:8000",
					TLSConfig: connectivityv1alpha1.RegistryTLSConfig{
						ServerName: "foobar.some.domain",
					},
				},
			},
			exportServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
			importServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain-6068d556",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ImportedLabel:                   "",
						connectivityv1alpha1.ConnectivityRemoteRegistryLabel: "shared-services-cls",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
		},
		{
			name: "random annotations should be passed into imported ServiceRecord",
			remoteRegistry: &connectivityv1alpha1.RemoteRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-services-cls",
					Namespace: "cross-cluster-connectivity",
				},
				Spec: connectivityv1alpha1.RemoteRegistrySpec{
					Address: "127.0.0.1:8000",
					TLSConfig: connectivityv1alpha1.RegistryTLSConfig{
						ServerName: "foobar.some.domain",
					},
				},
			},
			exportServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						"foobar":                                   "",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
			importServiceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar.some.domain-6068d556",
					Namespace: "cross-cluster-connectivity",
					Labels: map[string]string{
						connectivityv1alpha1.ImportedLabel:                   "",
						connectivityv1alpha1.ConnectivityRemoteRegistryLabel: "shared-services-cls",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "foobar.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
					},
				},
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			certBytes, keyBytes, err := generateCert("*.some.domain")
			if err != nil {
				t.Fatalf("error generating test certificates: %v", err)
			}

			testcase.remoteRegistry.Spec.TLSConfig.ServerCA = certBytes

			// create the client fake client with the test RemoteRegistry
			registryClientFake := fake.NewSimpleClientset(testcase.remoteRegistry)
			// create the server fake client with the exported ServiceRecord
			registryServerFake := fake.NewSimpleClientset(testcase.exportServiceRecord)

			registryClientInformerFactory := connectivityinformers.NewSharedInformerFactory(registryClientFake, 30*time.Second)
			registryServerInformerFactory := connectivityinformers.NewSharedInformerFactory(registryServerFake, 30*time.Second)

			registryClientController := newRegistryClientController(registryClientFake, registryClientInformerFactory)
			registryServerController, err := newRegistryServerController(registryServerFake, registryServerInformerFactory, certBytes, keyBytes)
			if err != nil {
				t.Fatalf("error creating server controller: %v", err)
			}

			registryClientInformerFactory.Start(nil)
			registryServerInformerFactory.Start(nil)

			registryClientInformerFactory.WaitForCacheSync(nil)
			registryServerInformerFactory.WaitForCacheSync(nil)

			go registryClientController.Run(1, nil)
			go registryServerController.Run(1, nil)

			go registryServerController.Server.Start()
			defer registryServerController.Server.Stop()

			checkImportedService := func() (bool, error) {
				importedServiceRecord, err := registryClientFake.ConnectivityV1alpha1().
					ServiceRecords(testcase.importServiceRecord.Namespace).
					Get(testcase.importServiceRecord.Name, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				if !reflect.DeepEqual(importedServiceRecord, testcase.importServiceRecord) {
					t.Logf("actual ServiceRecord: %v", importedServiceRecord)
					t.Logf("expected ServiceRecord: %v", testcase.importServiceRecord)
					return true, errors.New("ServiceRecords were not equal")
				}

				return true, err
			}

			if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkImportedService); err != nil {
				t.Errorf("ServiceRecord did not match: %v", err)
			}
		})
	}
}

func newRegistryClientController(fakeClient *fake.Clientset, informer connectivityinformers.SharedInformerFactory) *registryclient.RegistryClientController {
	remoteRegistryInformer := informer.Connectivity().V1alpha1().RemoteRegistries()
	serviceRecordInformer := informer.Connectivity().V1alpha1().ServiceRecords()

	return registryclient.NewRegistryClientController(fakeClient, remoteRegistryInformer, serviceRecordInformer, "cross-cluster-connectivity", 5*time.Minute)
}

func newRegistryServerController(fakeClient *fake.Clientset, informer connectivityinformers.SharedInformerFactory, tlsCert, tlsKey []byte) (*registryserver.RegistryServerController, error) {
	serviceRecordInformer := informer.Connectivity().V1alpha1().ServiceRecords()

	certFile, err := ioutil.TempFile("", "tls.crt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(certFile.Name())
	certFile.Write(tlsCert)

	keyFile, err := ioutil.TempFile("", "tls.key")
	if err != nil {
		return nil, err
	}
	defer os.Remove(keyFile.Name())
	keyFile.Write(tlsKey)

	r, err := registryserver.NewRegistryServerController(uint32(8000), certFile.Name(), keyFile.Name(), serviceRecordInformer)
	if err != nil {
		return nil, err
	}

	return r, nil
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
