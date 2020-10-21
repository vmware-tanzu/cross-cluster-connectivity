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
MIIEpAIBAAKCAQEAwIEPb5A+eyTHMVLJotTwGEJtQRsWS/QSLxxYcRbW2V4mPRbB
+PX+Y2YXDdTCC8QUtmPb60RwTZOJ1Gtwb5iZ0IVJtgtCGzLALammHv4bVGhNBE8D
u3NTdBPAX6/dfjGbQCvl651qK7Bij9gns8AxwblnrGa7x1/7vplk+mpe3Csx2/3A
hjW5E6IRVzbObKs63eH3e8m9AAXvVWxtwni44P4vpb8g92p+1H7VX5I0AJXeqNVz
rNmPbneXopzHDsl4zF6UHNjjFHL0RikJGu6fMFCCIGxx+2sb3lvLujz+4FQVUuIk
msfEhylcErt80rjhM3JAdlp3t+9Yi1RB/7SRSwIDAQABAoIBAAmVo24bkXDSKPTE
uXNZBMdAb24hah/H/CvKToD68SGLdX3vJyM9JDhQue8fW7X4Qku+dxGkq67BHMit
vMBhqa7fJAdjUhxGj5j2bGX4ouW197eyM25e1JXf8eERwYZp89/jD6SGhuW793xP
99IUTKXnlEjaKJlJpyAbRRLOvwBZNBDLlEOvILbO5t1kcS4Rrahqo65cS0g2uve6
dlFsHdPEEfXRnVxHNuQmJgQPp5sSWrNaeXKLsOEmxpVOVyIH4za4L7Y1zenzkEoY
UwOteZBfaqypYeEWvDRwsIcqehR5utju8NNnflUeAOp8bsCnGCdju7dROtIXpowo
gvll60kCgYEA7IyXmuQhamMDUlbK7ffnY0LwWNA1GkWxnedSvKec2iaJ2h0jkA4q
r5h9fAYvzzEo38YMiUSQwZpvnKSFdxLLXebarxPxYAHEHNH+jOD8M0oLGqaVyzfq
6ircTkElhUi0Is5Wci1u6aTpnc4RQ+90VglDBqJth0FuBwdyc/ct/UUCgYEA0FVP
oCjp1O6hSEaJVMPwCLX81M3yLrsc6hF9P4KzIdEi1yaSX2oHJsMaNNnXebNrme/L
cR0c/hc3T1NgymyTEpxxHChnJjNwm4pmLmYAIdl8AWY4XUrd2MwkK6fFtIQatGhR
gICfQAsZD+biJGd6JNIqlrcO5lEfl1sAjIe11U8CgYEA3T+65WMPZiRqDO+lKuM+
h3cqusczg/k/4kNk/ZOAgAKf2WR7yNeXUVo9tG1M9mwyoOrq+tEo3AyI7GhtdSwd
Dx1H2Y27rGK6fYJkpnwhKA/PRwQdA1Cv5opkOMVyRLH12sBH1s9r+BkJcVI2j+Y+
V+Kd0GzIKUQnl2d9w72kREkCgYBoMZKeTngMN8DQDf8XNtuw75vgrpOmTYy7gD28
6tg+XINpSXBBahzjhQZxUlYTFuoE1kpQazgZ2HCgKtoowz6XO0jSxV45W9bA4+oQ
4JDGXShI5t/fwNbNW+PnNYSKsNtOSTIh67I57JL/QgDuJhaPndERCcLY68+5+hh/
MEx/vwKBgQCDSvR9UlXcwJaurDdlCrQrk+vkdlA3cI6fkWIplqOfSBxK+NaKt8O6
tSDRC22P1ubxsSzPbkxR/7PK00A1qm+PRx76CS/meQtMd9GQCrbxVhrOHZbcn6jm
xVhR+uh0gujYSXG6L6yRpjPyIHw2u05d8DVGfH/3YIxFBvWXX0asNA==
-----END RSA PRIVATE KEY-----`
	tlsCert = `-----BEGIN CERTIFICATE-----
MIIEJDCCAgygAwIBAgIQAsqSUFUJAK+S/qcql2CPvzANBgkqhkiG9w0BAQsFADAR
MQ8wDQYDVQQDEwZmYWtlQ0EwHhcNMjAxMDE5MjI1NjMxWhcNMjIwNDE5MjI1NTUx
WjAYMRYwFAYDVQQDDA0qLnNvbWUuZG9tYWluMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAwIEPb5A+eyTHMVLJotTwGEJtQRsWS/QSLxxYcRbW2V4mPRbB
+PX+Y2YXDdTCC8QUtmPb60RwTZOJ1Gtwb5iZ0IVJtgtCGzLALammHv4bVGhNBE8D
u3NTdBPAX6/dfjGbQCvl651qK7Bij9gns8AxwblnrGa7x1/7vplk+mpe3Csx2/3A
hjW5E6IRVzbObKs63eH3e8m9AAXvVWxtwni44P4vpb8g92p+1H7VX5I0AJXeqNVz
rNmPbneXopzHDsl4zF6UHNjjFHL0RikJGu6fMFCCIGxx+2sb3lvLujz+4FQVUuIk
msfEhylcErt80rjhM3JAdlp3t+9Yi1RB/7SRSwIDAQABo3EwbzAOBgNVHQ8BAf8E
BAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB0GA1UdDgQWBBTb
8dF8Li7smC2L2JnTCe+dc1ZfyjAfBgNVHSMEGDAWgBQ0dSC4OpoNv0mILozxD6j0
sYRk1DANBgkqhkiG9w0BAQsFAAOCAgEAn/3YDYcHK8Q9KfgZRb4m26Pn9lumVIOP
l04kUnKaGkVPl2o5eNr/zYQBiqnTqb7BrGe0GE66p2i/4HYWQYapGKBkqNai+n2G
DpPDKUh+S+bIzBnbYBgCK44GV04Tqmg6omvqdxovXZfgRwZds3T4+o0uZsQY9d84
guw9shzSf81HLxw89NEFRR+UqK8nb4Vz3ZNbPePTo+WIubomN9Rq32VweTZGLZ1e
/i27zXPXTh4ZH0VPJQCYkH2gK5MYmYL7+9Lb6j7YO5lPWZjZD3q54GMN0UVDLncU
w0TYxJGSdra4hFAEUMCckPthGy91hBMIozwYLXBVQMQKb2LwX3njc2LU7EkP653e
E1BkWpLWfnWE38iEpHaNW21kTPqqNpX1rGh1XS0SvOZcrS1dQyHHmFJAo6LUyedi
ue1sduF3sxEwE7rZ9iRzAhHJLXgBjuV3ZELPEW5MWUcj0baT7XURZGC6hrcshgWY
NTzOabltJCdbuiJwqfZqu6urzAIO5d6qV1xCG9BXUysTFQ9MGCourrimeQwP8/WF
GCumVwc8rtfJDFrQx1kL0m2PvdC96LG9J2p1RaBu6nz6MTiTKETJSvrIHTCB42cP
HMxtwje9lbHeDsn/P7NUj+7zYAqrRQI6YOO/lVKE8xfKzYJiKDEUB3rlrT4oOUoM
LcJcvYQFnoU=
-----END CERTIFICATE-----`
	caCert = `-----BEGIN CERTIFICATE-----
MIIE4jCCAsqgAwIBAgIBATANBgkqhkiG9w0BAQsFADARMQ8wDQYDVQQDEwZmYWtl
Q0EwHhcNMjAxMDE5MjI1NTU2WhcNMjIwNDE5MjI1NTUyWjARMQ8wDQYDVQQDEwZm
YWtlQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDunPbhbBvL7ljM
Ceb/GEOaF9aTH3nBaqMuaQ/P5vI88GooM839O4PJagDPCMqi0WGVxvPamDAZsAmY
GLcuhWABxZDA7t2oOKpQbYz0/cjXGPnrW4DrdxofMCesvG8Z3QXmkWaQw/aqWJUy
X7NF1Myw73G/C81PUqP4Dz5xdvC8n/dg5ok/SmmJgInZyadXSBAgtFDxiP997IZC
Awrv23DnLl5h3ocfsDxZtum6JK8pIVfkHjoszPoc7m25Mxpmd8lUfRrSpn0rH/ZM
0Fe12p683eWHyCt0efoq83U//aTD2GUHNiCDxrmNpguqe9uinDe6CcCn0uzeL9o+
oMjGUWqpviJzyh2IckUy/LVn7ZxEXGwikWLRH/48RkgkiUcdu8JTL78ZcP2w7M3I
25F9hcwbVVnI01gKP2drpVUkt/5IBWuQ6XQ6Low0dBPbMu9PguI+EISr1rZ6kxqC
k1uNBWANbvD4lEjO3GGsSjyMxWs0NKcKL23A29Oa0NNbPG67pjX64Q4OJ2WmfAy2
cFb0i62tKnmctt2z8JyPhI9J9GcPQTPwmLYLPcStK3bY64vBv3c1WSpxgBRfbzTt
02hWO/M1B/dsxsemep3sUAgl83A/N+WIr6tQoDNjcdkeeujMhow29ReL4593ztbp
h89pcE506B3SggnSLVzLFo251S3LowIDAQABo0UwQzAOBgNVHQ8BAf8EBAMCAQYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUNHUguDqaDb9JiC6M8Q+o9LGE
ZNQwDQYJKoZIhvcNAQELBQADggIBABOgiPIYJEih93U92urCzXFmONNjWKa/ImmX
Gbet66e5i1e8eVaWz3d/c9EoEAZBninAp94uSkHb7uLshNL7lvbDjxwBJldAuuEQ
MvLN88fvPUg/kPCW8oQDhaaKbdqtyGxWAwLwFl5iljes3aiEtNUzpE8clUjrLw5r
h5FxVoUlm3MdzgfDQmahJ9ZmQBsA4Ab5LX71Va61i7Nf7KewQMuQ/zn6V8zNrA2q
F72IOgJKGVciqORmE9gX8iLoqAvlOBFoYqFMWAXdkjY8WlRsGfvVJzhjPaitaN+U
gcit+ZOTbMYcAA9eSiWXLGqRsvsLt+1zXLtf7YdUCS6GcDw9tsZb+jLf/+88qUkS
qCkoyp6U5mcD8O42656EhEBPmBUAGBgTTw2bZT95E3sxxiD5BcNGfSv8LnJHC/Bk
fbER2XeezEntMPZlADMPrV5ClrHTtErscfH2JvmmYVyFrum2qXg/F9hJDJ56WNcB
0BTvcTdWneqX8nBkUwMO2UKK/F5Zmt6fMQLupKmXwSGpX/VlHm14HpONJRRMLYW/
L2CbL657nb51ilcn77IJsyKT1jeCmg0ffUBRvQsa3GICh3S0UPVzqXKB++gu0qOC
NwO7W8kA0oVApcBRzyDDBhJevWzIJGxZY+KTcqqoBZwAFABqRmzjBlU8MbV2T0Hi
23gmSPQv
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
