// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registryclient_test

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/vmware/hamlet/pkg/server"

	log "github.com/sirupsen/logrus"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient"
	clientsetfake "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
	hamletv1alpha1 "github.com/vmware/hamlet/api/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

type fakeStateProvider struct {
	mutex           sync.RWMutex
	getStateReturns struct {
		message []proto.Message
		error   error
	}
}

func (f *fakeStateProvider) GetState(string) ([]proto.Message, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	return f.getStateReturns.message, f.getStateReturns.error
}

func (f *fakeStateProvider) GetStateReturns(message []proto.Message, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.getStateReturns.message = message
	f.getStateReturns.error = err
}

var _ = Describe("ClientController", func() {
	var (
		connClientset *clientsetfake.Clientset
		stateProvider *fakeStateProvider

		remoteRegistry *connectivityv1alpha1.RemoteRegistry
		hamletServer   server.Server

		deleteOrphanDelay time.Duration
	)

	BeforeEach(func() {
		log.SetOutput(GinkgoWriter)

		deleteOrphanDelay = 1 * time.Second

		stateProvider = &fakeStateProvider{}
		connClientset = clientsetfake.NewSimpleClientset()
		connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
		serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
		remoteRegistryInformer := connectivityInformerFactory.Connectivity().V1alpha1().RemoteRegistries()
		registryClientController := registryclient.NewRegistryClientController(
			connClientset,
			remoteRegistryInformer,
			serviceRecordInformer,
			"cross-cluster-connectivity",
			deleteOrphanDelay,
		)

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
				Name:       "some-remote-registry",
				Namespace:  "cross-cluster-connectivity",
				UID:        "remote-registry-uid",
				Generation: 1,
			},
			Spec: connectivityv1alpha1.RemoteRegistrySpec{
				Address: fmt.Sprintf("127.0.0.1:%d", hamletPort),
			},
		}

	})

	JustBeforeEach(func() {
		_, err := connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
			Create(remoteRegistry)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
			return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
				Get(remoteRegistry.Name, metav1.GetOptions{})
		}, 5*time.Second, time.Second).Should(MatchObservedGeneration(remoteRegistry.Generation))
	})

	AfterEach(func() {
		Expect(hamletServer.Stop()).To(Succeed())
	})

	It("sets a status on the RemoteRegistry indicating it's valid", func() {
		Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
			return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
				Get(remoteRegistry.Name, metav1.GetOptions{})
		}, 5*time.Second, time.Second).Should(ConsistOfStatusConditions(
			gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
				"Type":   Equal(connectivityv1alpha1.RemoteRegistryConditionValid),
				"Status": Equal(corev1.ConditionTrue),
			}),
		))
	})

	When("the remote registry server has a service record", func() {
		BeforeEach(func() {
			stateProvider.GetStateReturns([]proto.Message{
				dummyFederatedService(("some-service.some.domain")),
			}, nil)
		})

		It("creates a service record in the Kubernetes API", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				ConsistOfServiceRecords(MatchKubeObjectWithFields(gstruct.Fields{
					"Name": Equal("some-service.some.domain-06df7236"),
				})),
			)
		})

		It("sets a label on the service record with the registry name", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				ConsistOfServiceRecords(MatchKubeObjectWithFields(gstruct.Fields{
					"Labels": HaveKeyWithValue(connectivityv1alpha1.ConnectivityRemoteRegistryLabel, "some-remote-registry"),
				})),
			)
		})

		It("adds an OwnerReference to the RemoteRegistry on the created ServiceRecord", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				ConsistOfServiceRecords(MatchKubeObjectWithFields(gstruct.Fields{
					"OwnerReferences": ConsistOf(metav1.OwnerReference{
						APIVersion: "connectivity.tanzu.vmware.com/v1alpha1",
						Kind:       "RemoteRegistry",
						UID:        "remote-registry-uid",
						Name:       "some-remote-registry",
					}),
				})),
			)
		})
	})

	When("the remote registry server has multiple service records", func() {
		BeforeEach(func() {
			stateProvider.GetStateReturns([]proto.Message{
				dummyFederatedService(("some-service.some.domain")),
				dummyFederatedService(("other.some.domain")),
				dummyFederatedService(("other.somesome.domain")),
				dummyFederatedService(("some.other.domain")),
				dummyFederatedService(("domain.with.dot.at.end.")),
				dummyFederatedService(("domain.WITH.mixed.case")),
				dummyFederatedService(("yet.another.domain")),
			}, nil)
		})

		When("the RemoteRegistry has no domain filter", func() {
			BeforeEach(func() {
				remoteRegistry.Spec.AllowedDomains = []string{}
			})

			It("creates service records for all imported services", func() {
				Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
					return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
						List(metav1.ListOptions{})
				}, 5*time.Second, time.Second).Should(
					ConsistOfServiceRecords(
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some-service.some.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.some.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.somesome.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some.other.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.dot.at.end-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.mixed.case-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("yet.another.domain-06df7236")}),
					),
				)
			})

			When("the RemoteRegistry is updated to add a domain filter", func() {
				JustBeforeEach(func() {
					By("Ensuring that the controller has reconciled before the RemoteRegistry is updated")
					Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
						return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
							List(metav1.ListOptions{})
					}, 5*time.Second, time.Second).Should(WithTransform(transformServiceRecordListToItems, HaveLen(7)))

					By("Updating the RemoteRegistry to add the domain filter")
					remoteRegistry.Spec.AllowedDomains = []string{"some.domain"}
					remoteRegistry.ObjectMeta.Generation = 42
					_, err := connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
						Update(remoteRegistry)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
						return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
							Get(remoteRegistry.Name, metav1.GetOptions{})
					}, 5*time.Second, time.Second).Should(MatchObservedGeneration(42))
				})

				It("service records with non-matching domains should be deleted, and only service records for services with matching domains should remain", func() {
					Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
						return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
							List(metav1.ListOptions{})
					}, 5*time.Second, time.Second).Should(
						ConsistOfServiceRecords(
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some-service.some.domain-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.some.domain-06df7236")}),
						),
					)
				})
			})
		})

		When("the RemoteRegistry has domain filters", func() {
			BeforeEach(func() {
				remoteRegistry.Spec.AllowedDomains = []string{
					"some.domain",
					"some.other.domain",
					"domain.with.dot.at.end.",
					"domain.with.MIXED.case",
				}
			})

			It("only creates service records for services with matching domains", func() {
				Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
					return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
						List(metav1.ListOptions{})
				}, 5*time.Second, time.Second).Should(
					ConsistOfServiceRecords(
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some-service.some.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.some.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some.other.domain-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.dot.at.end-06df7236")}),
						MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.mixed.case-06df7236")}),
					),
				)
			})

			When("the RemoteRegistry is updated to remove the domain filter", func() {
				JustBeforeEach(func() {
					By("Ensuring that the controller has reconciled before the RemoteRegistry is updated")
					Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
						return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
							List(metav1.ListOptions{})
					}, 5*time.Second, time.Second).Should(WithTransform(transformServiceRecordListToItems, HaveLen(5)))

					By("Updating the RemoteRegistry to remove the domain filter")
					remoteRegistry.Spec.AllowedDomains = []string{}
					remoteRegistry.ObjectMeta.Generation = 42
					_, err := connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
						Update(remoteRegistry)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
						return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
							Get(remoteRegistry.Name, metav1.GetOptions{})
					}, 5*time.Second, time.Second).Should(MatchObservedGeneration(42))
				})

				It("creates service records for all imported services", func() {
					Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
						return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
							List(metav1.ListOptions{})
					}, 5*time.Second, time.Second).Should(
						ConsistOfServiceRecords(
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some-service.some.domain-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.some.domain-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("other.somesome.domain-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("some.other.domain-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.dot.at.end-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("domain.with.mixed.case-06df7236")}),
							MatchKubeObjectWithFields(gstruct.Fields{"Name": Equal("yet.another.domain-06df7236")}),
						),
					)
				})
			})
		})
	})

	When("the remote registry server has a service record with a longer than 63 character fqdn", func() {
		var fqdn string
		BeforeEach(func() {
			fqdnLabel := strings.Repeat("a", 63)
			fqdn = fmt.Sprintf("%s.some.domain", fqdnLabel)
			stateProvider.GetStateReturns([]proto.Message{dummyFederatedService(fqdn)}, nil)
		})

		It("creates a service record in the Kubernetes API", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				ConsistOfServiceRecords(MatchKubeObjectWithFields(gstruct.Fields{
					"Name": Equal(fmt.Sprintf("%s-06df7236", fqdn)),
				})),
			)
		})
	})

	When("there is a federated service for a record and an existing orphaned record", func() {
		BeforeEach(func() {
			federatedService := &hamletv1alpha1.FederatedService{
				Name:      "some-service.some.domain",
				Fqdn:      "some-service.some.domain",
				Id:        "some-service.some.domain",
				Protocols: []string{"https"},
				Endpoints: []*hamletv1alpha1.FederatedService_Endpoint{},
			}

			anotherFederatedService := &hamletv1alpha1.FederatedService{
				Name:      "another-service.some.domain",
				Fqdn:      "another-service.some.domain",
				Id:        "another-service.some.domain",
				Protocols: []string{"https"},
				Endpoints: []*hamletv1alpha1.FederatedService_Endpoint{},
			}

			stateProvider.GetStateReturns([]proto.Message{federatedService, anotherFederatedService}, nil)

			_, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				Create(&connectivityv1alpha1.ServiceRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-service.some.domain-06df7236",
						Namespace: "cross-cluster-connectivity",
						UID:       "1234",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "some-service.some.domain",
					},
				})
			Expect(err).NotTo(HaveOccurred())

			_, err = connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				Create(&connectivityv1alpha1.ServiceRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "orphaned-service-record-06df7236",
						Namespace: "cross-cluster-connectivity",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "orphaned-service.some.domain",
					},
				})
			Expect(err).NotTo(HaveOccurred())
		})

		It("eventually deletes the orphaned record and not the record added by the remote registry", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 10*time.Second, time.Second).Should(
				ConsistOfServiceRecords(
					MatchKubeObjectWithFields(gstruct.Fields{
						"Name": Equal("some-service.some.domain-06df7236"),
						"UID":  Equal(apimachinerytypes.UID("1234")), // the uid proves this is the original ServiceRecord
					}),
					MatchKubeObjectWithFields(gstruct.Fields{
						"Name": Equal("another-service.some.domain-06df7236"),
					}),
				),
			)
		})
	})

	When("the RemoteRegistry is created with an invalid AllowedDomains list", func() {
		BeforeEach(func() {
			stateProvider.GetStateReturns([]proto.Message{
				dummyFederatedService(("some-service.some.domain")),
			}, nil)

			remoteRegistry.Spec.AllowedDomains = []string{
				"invalid..domain",
			}
		})

		It("does not create a service record in the Kubernetes API", func() {
			Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
				return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
			}, 5*time.Second, time.Second).Should(
				WithTransform(transformServiceRecordListToItems, BeEmpty()),
			)
		})

		It("sets a status on the RemoteRegistry resource indicating the validation error", func() {
			Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
				return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
					Get(remoteRegistry.Name, metav1.GetOptions{})
			}, 5*time.Second, time.Second).Should(ConsistOfStatusConditions(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":    Equal(connectivityv1alpha1.RemoteRegistryConditionValid),
					"Status":  Equal(corev1.ConditionFalse),
					"Reason":  Equal("InvalidDomain"),
					"Message": ContainSubstring("Invalid domain"),
				}),
			))
		})
	})

	When("a previously valid RemoteRegistry is updated with an invalid AllowedDomains list", func() {
		BeforeEach(func() {
			stateProvider.GetStateReturns([]proto.Message{
				dummyFederatedService(("some-service.some.domain")),
			}, nil)

			remoteRegistry.Spec.AllowedDomains = []string{
				"some.domain",
			}
		})

		JustBeforeEach(func() {
			By("Ensuring that the controller has reconciled before the RemoteRegistry is updated")
			Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
				return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
					Get(remoteRegistry.Name, metav1.GetOptions{})
			}, 5*time.Second, time.Second).Should(ConsistOfStatusConditions(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":   Equal(connectivityv1alpha1.RemoteRegistryConditionValid),
					"Status": Equal(corev1.ConditionTrue),
				}),
			))

			By("Updating the RemoteRegistry to add an invalid domain filter")
			remoteRegistry.Spec.AllowedDomains = []string{"invalid..domain"}
			remoteRegistry.ObjectMeta.Generation = 42

			_, err := connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
				Update(remoteRegistry)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
				return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
					Get(remoteRegistry.Name, metav1.GetOptions{})
			}, 5*time.Second, time.Second).Should(MatchObservedGeneration(42))
		})

		It("sets a status on the RemoteRegistry indicating the validation error", func() {
			Eventually(func() (*connectivityv1alpha1.RemoteRegistry, error) {
				return connClientset.ConnectivityV1alpha1().RemoteRegistries("cross-cluster-connectivity").
					Get(remoteRegistry.Name, metav1.GetOptions{})
			}, 5*time.Second, time.Second).Should(ConsistOfStatusConditions(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Type":    Equal(connectivityv1alpha1.RemoteRegistryConditionValid),
					"Status":  Equal(corev1.ConditionFalse),
					"Reason":  Equal("InvalidDomain"),
					"Message": ContainSubstring("Invalid domain"),
				}),
			))
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

func ConsistOfServiceRecords(serviceRecords ...interface{}) types.GomegaMatcher {
	return WithTransform(transformServiceRecordListToItems, ConsistOf(serviceRecords...))
}

func MatchKubeObjectWithFields(objectMetaFields gstruct.Fields) types.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, objectMetaFields),
	})
}

func MatchObservedGeneration(generationNum int64) types.GomegaMatcher {
	return WithTransform(func(remoteRegistry *connectivityv1alpha1.RemoteRegistry) int64 {
		return remoteRegistry.Status.ObservedGeneration
	}, Equal(generationNum))
}

func ConsistOfStatusConditions(conditions ...interface{}) types.GomegaMatcher {
	return gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Conditions": ConsistOf(conditions...),
		}),
	}))
}

func transformServiceRecordListToItems(srl *connectivityv1alpha1.ServiceRecordList) []connectivityv1alpha1.ServiceRecord {
	if srl == nil || len(srl.Items) == 0 {
		return nil
	}
	return srl.Items
}

func dummyFederatedService(fqdn string) *hamletv1alpha1.FederatedService {
	return &hamletv1alpha1.FederatedService{
		Name:      fqdn,
		Fqdn:      fqdn,
		Id:        fqdn,
		Protocols: []string{"https"},
		Endpoints: []*hamletv1alpha1.FederatedService_Endpoint{},
	}
}
