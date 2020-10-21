// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registryclient_test

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/vmware/hamlet/pkg/server"

	log "github.com/sirupsen/logrus"
	hamletv1alpha1 "github.com/vmware/hamlet/api/types/v1alpha1"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient"
	clientsetfake "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

type fakeStateProvider struct {
	getStateReturns struct {
		message []proto.Message
		error   error
	}
}

func (f *fakeStateProvider) GetState(string) ([]proto.Message, error) {
	return f.getStateReturns.message, f.getStateReturns.error
}

func (f *fakeStateProvider) GetStateReturns(message []proto.Message, err error) {
	f.getStateReturns.message = message
	f.getStateReturns.error = err
}

var _ = Describe("ClientController", func() {
	var (
		connClientset *clientsetfake.Clientset
		stateProvider *fakeStateProvider

		remoteRegistry *connectivityv1alpha1.RemoteRegistry
		hamletServer   server.Server
	)

	BeforeEach(func() {
		log.SetOutput(GinkgoWriter)

		stateProvider = &fakeStateProvider{}
		connClientset = clientsetfake.NewSimpleClientset()
		connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
		serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
		remoteRegistryInformer := connectivityInformerFactory.Connectivity().V1alpha1().RemoteRegistries()
		registryClientController := registryclient.NewRegistryClientController(connClientset,
			remoteRegistryInformer,
			serviceRecordInformer)

		connectivityInformerFactory.Start(nil)
		connectivityInformerFactory.WaitForCacheSync(nil)

		go registryClientController.Run(1, nil)

		var err error
		hamletPort := randomPort()
		hamletServer, err = server.NewServer(uint32(hamletPort), nil, stateProvider)
		Expect(err).NotTo(HaveOccurred())

		go hamletServer.Start()

		remoteRegistry = &connectivityv1alpha1.RemoteRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-remote-registry",
				Namespace: "cross-cluster-connectivity",
			},
			Spec: connectivityv1alpha1.RemoteRegistrySpec{
				Address: fmt.Sprintf("127.0.0.1:%d", hamletPort),
			},
		}

		_, err = connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
			Create(remoteRegistry)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(hamletServer.Stop()).To(Succeed())
	})

	When("the remote registry server has a service record", func() {
		BeforeEach(func() {
			federatedService := &hamletv1alpha1.FederatedService{
				Name:      "some-service.some.domain",
				Fqdn:      "some-service.some.domain",
				Id:        "some-service.some.domain",
				Protocols: []string{"https"},
				Endpoints: []*hamletv1alpha1.FederatedService_Endpoint{},
			}
			stateProvider.GetStateReturns([]proto.Message{federatedService}, nil)
		})

		It("creates a service record in the Kubernetes API", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				MatchServiceRecord(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("some-service.some.domain-06df7236"),
					}),
				})),
			)
		})

		It("sets a label on the service record with the registry name", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				MatchServiceRecord(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Labels": HaveKeyWithValue(connectivityv1alpha1.ConnectivityRemoteRegistryLabel, "some-remote-registry"),
					}),
				})),
			)
		})
	})

	When("the remote registry server has a service record with a longer than 63 character fqdn", func() {
		var fqdn string
		BeforeEach(func() {
			fqdnLabel := strings.Repeat("a", 63)
			fqdn = fmt.Sprintf("%s.some.domain", fqdnLabel)
			federatedService := &hamletv1alpha1.FederatedService{
				Name:      fqdn,
				Fqdn:      fqdn,
				Id:        fqdn,
				Protocols: []string{"https"},
				Endpoints: []*hamletv1alpha1.FederatedService_Endpoint{},
			}
			stateProvider.GetStateReturns([]proto.Message{federatedService}, nil)
		})

		It("creates a service record in the Kubernetes API", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				MatchServiceRecord(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal(fmt.Sprintf("%s-06df7236", fqdn)),
					}),
				})),
			)
		})
	})
})

func randomPort() int {
	tempListener, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())
	listenerAddr := tempListener.Addr().String()
	Expect(tempListener.Close()).To(Succeed())
	addr, err := net.ResolveTCPAddr("tcp", listenerAddr)
	Expect(err).NotTo(HaveOccurred())

	return addr.Port
}

func MatchServiceRecord(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(transformServiceRecordListToItems, And(HaveLen(1), ContainElement(matcher)))
}

func transformServiceRecordListToItems(srl *connectivityv1alpha1.ServiceRecordList) []connectivityv1alpha1.ServiceRecord {
	if srl == nil || len(srl.Items) == 0 {
		return nil
	}
	return srl.Items
}
