// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"io/ioutil"
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

const (
	tlsKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAv13py5t5lkJPH+9qiOimx92wGGlzPV8mdONjkNbOOMQ4RGpj
0yGDfHYyiKJtUyUwhcS4Ek+boZN8fY8gmFZoTlW9Q84p3ETa8+LGZ8WES9mIq2TA
42Vmh9pPKq71RZ/btMP20sORGW9y5Y17FzqUZt5RhMaQZyo4CX+sWs07vVYKQbaV
WywSXR4lM8CcxkUvXmuKcSTM5tWRrYJqKJ5RQ50xttDUyuGul95XoT8gfZwuoo6Q
IpA7g4lEfco2MGgsXdvCAitit6OipcCPVeJwD184+XnKDh/ej139L7eHxcdLnCA4
akzbTYs41EPOkLp9Wd6fqaZrRG8LqTl9kAjRkQIDAQABAoIBAAH/TO1fFgnHb2P5
77a2FueHHMtkblt5nsEhjmx4kXZuNdgg9CHD+8dUxHpAl7uCa9s5jmJCinFJRMda
sxBj9nq60lrez/kIjvB0sXVrzlGsV4zSZGD7MfLBCIp9gPnVDUn5sZ3JhL4rN5vF
uj8n0VyxfVBRcjhhbGxM9NONyM3VD26q/suWCUVRS4BarwZ4wLmA5mp1suNvh/Td
cUtM0IpzJwehktdl2fo5zAla/kqztXlwC2mdsUySYu8fmEynrvwvNQsBKNKHBtF/
U1xRchFg3BbNaY2WZgF6/0uNxECoHPPzLAQFDTcRz6w3rj3pQWM/CKWROeZN6Rpr
LMZM2PECgYEA5i8p+ryJGMB1ESP4StVsSlBZ96X4VUdXOWHMyp5bZI3gih6XF0+w
uQj0GjiuzeoVGhBkezRQbAli98Fr07z0kThnIvRg8s6gdpOg2edTGmTgZy6BCkG5
uvLG0UXnxS3vcb4uYSdzlHqhHrC/NDl0TPYq/C5kCJytpYOOvv8atGMCgYEA1NRC
sxqVLkBg6Ya7FCDWM441iI3ALFg5tPdZflOT04jwf6D7b0UJhT+nhZXW0TF4CyjM
8eBcqTPnm4QHxSoZ6opvq7MkU5U7jq6DsRLvwGYIAFQkCAhM105d/gvY0KgslPpX
p6hAHNxC5nqXJPklhAm4K32NcE4Af3lmLkt1InsCgYEAxIasHslte9aFnNbLHIlP
ZbtotMndVmIMlI9tm+jMOvPvK72mXl7JkZGVZ/XROTmMPq6UO6SUrUjuWH2ppCQF
4x72358qTuQfmF2+zYx1JWnPNgk8XxdyjazOFsrKcU0gzEoFqylVwwVYHq3k8Z/E
LhlW5extt/SdRV0nOObxU+UCgYEAgHq+5TZX9nrgxjkSeJ02Eht4T74a0+pSs99a
RDuaEuopHTMGdm57x9fcfnUtIE43xKzVw/KInZB68dPriOfYi1EVBtb3SAnf0Uui
rmPbHg+6JtCki8DO+m8RqMpoEdZkS28xOUIFqiaBsHczBRvuvN3NM1vw5WoBPPMB
b1MYHD8CgYEAnzolLttSoo+0+sUsnboDwekL6AzI97BVvHG4R1oWwsCT5Y811tlh
Xok+JJEWvj0Q9Xv8yOXLkNtk0sqO55CmuzbV0gu62WPGe1XODL0hwCBvO9p3q8ve
LNvt6NomFMrQ4xFjVyIspsetSU/zj9dhIfN6q3150nnBgQTP71qxWOo=
-----END RSA PRIVATE KEY-----`
	tlsCert = `-----BEGIN CERTIFICATE-----
MIIEQTCCAimgAwIBAgIRAITRmC5AvykwQPiGNc15+GQwDQYJKoZIhvcNAQELBQAw
ETEPMA0GA1UEAxMGZmFrZUNBMB4XDTIwMTAyMjE2NTU1NVoXDTIyMDQyMjE2Mzkw
NlowGDEWMBQGA1UEAwwNKi5zb21lLmRvbWFpbjCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAL9d6cubeZZCTx/vaojopsfdsBhpcz1fJnTjY5DWzjjEOERq
Y9Mhg3x2MoiibVMlMIXEuBJPm6GTfH2PIJhWaE5VvUPOKdxE2vPixmfFhEvZiKtk
wONlZofaTyqu9UWf27TD9tLDkRlvcuWNexc6lGbeUYTGkGcqOAl/rFrNO71WCkG2
lVssEl0eJTPAnMZFL15rinEkzObVka2CaiieUUOdMbbQ1MrhrpfeV6E/IH2cLqKO
kCKQO4OJRH3KNjBoLF3bwgIrYrejoqXAj1XicA9fOPl5yg4f3o9d/S+3h8XHS5wg
OGpM202LONRDzpC6fVnen6mma0RvC6k5fZAI0ZECAwEAAaOBjDCBiTAOBgNVHQ8B
Af8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQW
BBQo17PEr97bdcpD/7MgszueDwKY1DAfBgNVHSMEGDAWgBR2NsYsdS+tBCU0xl00
f7yQvWCWCTAYBgNVHREEETAPgg0qLnNvbWUuZG9tYWluMA0GCSqGSIb3DQEBCwUA
A4ICAQAViOvGxjTJ07lbxnmfC7fjf+EHVpjsgh1tGYgOQSpLOkrh+WMLLdYep9+y
A69Uj/7ilRmWYTY+cJ9TuJ6ibTuu9PaTra31SK5RLj2Uxu2b+dQtk9qog9kXo/32
WLIjPHTMfo94TLKwjb5P5XHokfY6PIctg+i4oxxA0fCq32eiEj6+r+1zvW/9U5mS
oi6k0wexRb04AWP6IuIrE1VXEODyYP9dK5tcdhYO0/H9DviIwbcDQR05s6Qt8t1W
ZX+x2e/qRS9S2MyqSEpnlKcBiz1j34jYUDDqNZOYUy27Wr4Ucd3HPsq+57G91hm3
sdSQcn/FgUg/eUixfpYid3dzF2P/JEPfdG3WD68oGvg+2XOPRQRsDokPGrOlRPWe
kwOwLgZjB7mGqQA7OPZNJmn6/3AjGAoE7bczmiIgjVLbnlQWEecwcTC8MHUpKPAI
qjuRKz6gPvPF5TLtWSv3SHmdoPzlXf0+p/8awby9MuSdiXzxzlHRl5OUScTk2pTB
LmVlKpNFUX+ajLrdUB406y3UZ5iF6KaVL2gZZrd/gpvqOPlZJPPyUoJmcgbZ0ZSK
oDopZ7iCEIkSag4jTKv3SscwqyuupCXXuZ2BqEpR/6BZ5rj223Mz4zcebN0LcOON
yEIXPm9UN+3aYOntow0yl4DidTMLawghFgjZp2MyIlJrPvMWbw==
-----END CERTIFICATE-----`
	caCert = `-----BEGIN CERTIFICATE-----
MIIE4jCCAsqgAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZmYWtl
Q0EwHhcNMjAxMDIyMTYzOTE2WhcNMjIwNDIyMTYzOTA3WjARMQ8wDQYDVQQDEwZm
YWtlQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCs06RzbD+8FonM
e2SFsYlc5k5yfa+4r0S7amvITUn2tn1RlauHaILBhBg4w2m+wSdrAtoibAIXH+9q
8asOBGlS754sQkCwggm3GpUt5EkrFThFbbz5nCowjCrql+O0p0Cs7xdkVq3MZSVD
TskjKg9FUAcFPRQGAO3cE3lx++WO7LHXen0ykEUQF2Zfeq1QEAxLvwopWFQ6eBVp
gtcACXSUeo4HbPJDJXgdarThfhCvBLzO3ZnbxpD9M4Iy9rzzLgQ1sCaLvDHTCysI
copyGKpWsgQfgW0KdR9mdI3m0t69QUsFvQxCq8vaeXW6lC+7GyyGSkaUQBF/fPhj
o6lFGh2wejglBbpCHPbmlgcWzN3lFK6fhXJoWmmdE8ol1RBItV16U+b+1oOfnd7h
Z/wWJT/K6yZFiJfxrNfiJLvT1oiJ92jj+LfvExbBm1tC26eV3NiIcTfCKM8Vr1AC
c7KO1crExZQcI65crpLPuoODvEgYDUb6Nfqriqg6TU0yJxNne5N0cHzfytIA6ek0
SS2lC/s0H/5EFkZbJBOFEYjvGmYWsx9o6vWehwZhpiOqPxjpZTdHroZ7iPYs76oB
v2Qah9rSXV4m54GjjKOvr5qt5bP2Ebk+46gxok3De2RnqKjCVuQzEjqyG5MgHyQV
gorS3aDMPP0ipQ/B8oz1YDiUC35ysQIDAQABo0UwQzAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUdjbGLHUvrQQlNMZdNH+8kL1g
lgkwDQYJKoZIhvcNAQELBQADggIBAAl6AyjxmWwjNTnxWu9/+X0HeHcBMvH0WLo7
dWnCGuArdSQC6xzdIjC+m7Yxzcwwbjo3FY0WAoFufBZYbasmNcB5REUWuzC/Rdem
tEvUUgXYeA33vPYYwv6OFRVMNoQKEfhauPoBlgviO2IR8VCNmko7Ep8r/MBWM6kf
2P10GiC5qEtfWXMne4icQXIaM8+G7iJrwJlv9SPYHsYmGOPe0EzhQAc1ASfXH6Ty
knUVUM12F3WARZa50Tab5368c8pbKFLXYJpNK75e//A8SgoeKq/F/TjS+PlvTAvd
eLYJsw+JZu/zfQDif0V1PCjp4FhCQLB9fx4RlRi6/7lInMKG0A+GQ0qezm9hAdVj
kBtUA6Iqxn5K2T+nBpO9SnMym0Z0I9VolHeY/ufiZ3hUM4I1NZxsE8NDqZxd+fQh
VojtIousG3fIWP4O1gCLyWalTgBEMqtHRu+9x3tUbN7qQfhD4UuHZCH/epODGSP3
MuK6Gg5KiP/anGj4UfK5DqA2bnmY58jFlVRRXWyMcUUJjekUy5aKBQYeyxjuWKHx
C6v4tm8GJC/cX1wLrjMtyZWUcgr9kLbqA1W0+W8SoUIJomKjuTyJhaqpGzw1dvsX
7n9oqojdeUuTahg4e4HXMehHj/1XLDhfTNslIHw9bBRmtDSvxqjAnckPuVYEqq/7
Wq/UgGBe
-----END CERTIFICATE-----`
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
						ServerCA:   []byte(caCert),
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
						ServerCA:   []byte(caCert),
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
						ServerCA:   []byte(caCert),
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
			// create the client fake client with the test RemoteRegistry
			registryClientFake := fake.NewSimpleClientset(testcase.remoteRegistry)
			// create the server fake client with the exported ServiceRecord
			registryServerFake := fake.NewSimpleClientset(testcase.exportServiceRecord)

			registryClientInformerFactory := connectivityinformers.NewSharedInformerFactory(registryClientFake, 30*time.Second)
			registryServerInformerFactory := connectivityinformers.NewSharedInformerFactory(registryServerFake, 30*time.Second)

			registryClientController := newRegistryClientController(registryClientFake, registryClientInformerFactory)
			registryServerController, err := newRegistryServerController(registryServerFake, registryServerInformerFactory)
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

	return registryclient.NewRegistryClientController(fakeClient, remoteRegistryInformer, serviceRecordInformer)
}

func newRegistryServerController(fakeClient *fake.Clientset, informer connectivityinformers.SharedInformerFactory) (*registryserver.RegistryServerController, error) {
	serviceRecordInformer := informer.Connectivity().V1alpha1().ServiceRecords()

	certFile, err := ioutil.TempFile("", "tls.crt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(certFile.Name())
	certFile.Write([]byte(tlsCert))

	keyFile, err := ioutil.TempFile("", "tls.key")
	if err != nil {
		return nil, err
	}
	defer os.Remove(keyFile.Name())
	keyFile.Write([]byte(tlsKey))

	r, err := registryserver.NewRegistryServerController(uint32(8000), certFile.Name(), keyFile.Name(), serviceRecordInformer)
	if err != nil {
		return nil, err
	}

	return r, nil
}
