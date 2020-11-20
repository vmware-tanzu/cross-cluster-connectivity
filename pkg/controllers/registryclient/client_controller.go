// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registryclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
	"time"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
	hamletv1alpha1 "github.com/vmware/hamlet/api/types/v1alpha1"
	"github.com/vmware/hamlet/pkg/client"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient/internal/orphandeleter"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

var tlsOverride = false

type registryClient struct {
	remoteRegistry *connectivityv1alpha1.RemoteRegistry
	connClientSet  connectivityclientset.Interface

	serviceRecordLister connectivitylisters.ServiceRecordLister
	orphanDeleter       *orphandeleter.OrphanDeleter

	// sending to this channel forces a redial of the client
	// for when the RemoteRegistry resource changes.
	reDial chan struct{}
	// send to this channel when RemoteRegistry resource is deleted
	stopCh chan struct{}

	namespace      string
	allowedDomains []string
}

func newRegistryClient(
	remoteRegistry *connectivityv1alpha1.RemoteRegistry,
	connClientSet connectivityclientset.Interface,
	serviceRecordLister connectivitylisters.ServiceRecordLister,
	namespace string,
	deleteOrphanDelay time.Duration,
) *registryClient {
	return &registryClient{
		remoteRegistry:      remoteRegistry,
		connClientSet:       connClientSet,
		serviceRecordLister: serviceRecordLister,
		reDial:              make(chan struct{}),
		stopCh:              make(chan struct{}),
		namespace:           namespace,
		orphanDeleter:       orphandeleter.NewOrphanDeleter(serviceRecordLister, connClientSet, namespace, deleteOrphanDelay),
		allowedDomains:      cloneStringSlice(remoteRegistry.Spec.AllowedDomains),
	}
}

func (r *registryClient) run() {
	// this ticker enforces a 1 second delay between retries
	ticker := time.NewTicker(1 * time.Second)
	logger := log.WithField("remoteregistry", r.registryKey())
	for {
		select {
		case <-ticker.C:
			logger.Infof("creating hamlet client")
			client, err := r.newHamletClient()
			if err != nil {
				logger.Errorf("error creating hamlet client: %v", client)
				continue
			}

			streamEndedNotify := make(chan struct{}, 1)
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Infof("watching federated services")
				err = client.WatchFederatedServices(ctx, r)
				if err != nil && err != io.EOF {
					log.WithField("err", err).Errorf("Error occurred while watching federated services")
				}

				streamEndedNotify <- struct{}{}
			}()

			select {
			case <-r.reDial:
				logger.Infof("redial received for RemoteRegistry %s/%s",
					r.remoteRegistry.Namespace, r.remoteRegistry.Name)
				cancel()
				r.orphanDeleter.Reset()
			case <-streamEndedNotify:
				logger.Info("stream ended")
				r.orphanDeleter.Reset()
			case <-r.stopCh:
				logger.Infof("stop signal received")
				cancel()
				r.orphanDeleter.Reset()
				return
			}

		}
	}
}

func (r *registryClient) redial(registry *connectivityv1alpha1.RemoteRegistry) {
	// update the remote registry field and send redial signal
	r.remoteRegistry = registry
	r.allowedDomains = cloneStringSlice(registry.Spec.AllowedDomains)
	r.reDial <- struct{}{}
}

func (r *registryClient) stop() {
	// send stop signal to registry client
	r.stopCh <- struct{}{}
}

func (r *registryClient) registryKey() string {
	return r.remoteRegistry.Namespace + "/" + r.remoteRegistry.Name
}

func (r *registryClient) newHamletClient() (client.Client, error) {
	var tlsConfig *tls.Config
	if !tlsOverride {
		certPools := x509.NewCertPool()
		certPools.AppendCertsFromPEM(r.remoteRegistry.Spec.TLSConfig.ServerCA)
		tlsConfig = &tls.Config{
			RootCAs:    certPools,
			ServerName: r.remoteRegistry.Spec.TLSConfig.ServerName,
		}
	} else {
		log.Warn("Skipping TLS configuration for unit tests")
	}

	client, err := client.NewClient(r.remoteRegistry.Spec.Address, tlsConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (r *registryClient) syncHamletService(f *hamletv1alpha1.FederatedService) error {
	desiredServiceRecord := r.convertToKubernetesServiceRecord(f)

	log.Infof("sync hamlet service: %s/%s", desiredServiceRecord.Namespace, desiredServiceRecord.Name)

	r.orphanDeleter.AddRemoteServiceRecord(types.NamespacedName{
		Name:      desiredServiceRecord.Name,
		Namespace: desiredServiceRecord.Namespace,
	})

	currentServiceRecord, err := r.serviceRecordLister.ServiceRecords(desiredServiceRecord.Namespace).Get(desiredServiceRecord.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if !r.isDomainAllowed(desiredServiceRecord) {
				// do not create a non-allowed domain
				log.Infof("creation of %s/%s skipped due to non-allowed domain", desiredServiceRecord.Namespace, desiredServiceRecord.Name)
				return nil
			}
			log.Infof("creating: %s/%s", desiredServiceRecord.Namespace, desiredServiceRecord.Name)

			_, err = r.connClientSet.ConnectivityV1alpha1().ServiceRecords(desiredServiceRecord.Namespace).
				Create(desiredServiceRecord)
			return err
		}

		return fmt.Errorf("error getting current ServiceRecord: %v", err)
	}

	if !r.isDomainAllowed(desiredServiceRecord) {
		// exists, but shouldn't because the domain is no longer allowed
		log.Infof("deleting %s/%s due to non-allowed domain", currentServiceRecord.Namespace, currentServiceRecord.Name)

		err = r.connClientSet.ConnectivityV1alpha1().ServiceRecords(currentServiceRecord.Namespace).
			Delete(currentServiceRecord.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting ServiceRecord: %v", err)
		}
		return nil
	}

	// exists, update if current state does not match desired state
	newServiceRecord := currentServiceRecord.DeepCopy()
	newServiceRecord.Labels = desiredServiceRecord.Labels
	newServiceRecord.Annotations = desiredServiceRecord.Annotations
	newServiceRecord.Spec = desiredServiceRecord.Spec

	if apiequality.Semantic.DeepEqual(currentServiceRecord, newServiceRecord) {
		return nil
	}

	_, err = r.connClientSet.ConnectivityV1alpha1().ServiceRecords(newServiceRecord.Namespace).Update(newServiceRecord)
	if err != nil {
		return fmt.Errorf("error updating ServiceRecord: %v", err)
	}

	return nil
}

func (r *registryClient) deleteHamletService(f *hamletv1alpha1.FederatedService) error {
	return r.deleteServiceRecord(r.convertToKubernetesServiceRecord(f))
}

func (r *registryClient) deleteServiceRecord(serviceRecord *connectivityv1alpha1.ServiceRecord) error {
	currentServiceRecord, err := r.serviceRecordLister.ServiceRecords(serviceRecord.Namespace).Get(serviceRecord.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted
			return nil
		}

		return fmt.Errorf("error getting current ServiceRecord: %v", err)
	}

	return r.connClientSet.ConnectivityV1alpha1().ServiceRecords(currentServiceRecord.Namespace).Delete(currentServiceRecord.Name, &metav1.DeleteOptions{})
}

// OnCreate handler from Hamlet server
func (r *registryClient) OnCreate(f *hamletv1alpha1.FederatedService) error {
	log.WithField("federatedservice", f).Info("OnCreate FederatedService")

	return r.syncHamletService(f)
}

// OnUpdate handler from Hamlet server
func (r *registryClient) OnUpdate(f *hamletv1alpha1.FederatedService) error {
	log.WithField("federatedservice", f).Info("OnUpdate FederatedService")

	return r.syncHamletService(f)
}

// OnDelete handler from Hamlet server
func (r *registryClient) OnDelete(f *hamletv1alpha1.FederatedService) error {
	log.WithField("federatedservice", f).Info("OnDelete FederatedService")

	return r.deleteHamletService(f)
}

func (r *registryClient) convertToKubernetesServiceRecord(fs *hamletv1alpha1.FederatedService) *connectivityv1alpha1.ServiceRecord {
	endpoints := []connectivityv1alpha1.Endpoint{}
	for _, endpoint := range fs.Endpoints {
		endpoints = append(endpoints, connectivityv1alpha1.Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
		})
	}

	fqdn := strings.TrimSuffix(dns.CanonicalName(fs.Fqdn), ".")
	serviceRecord := &connectivityv1alpha1.ServiceRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateServiceRecordName(fqdn, r.remoteRegistry.Name),
			Namespace: r.namespace,
			Labels: map[string]string{
				connectivityv1alpha1.ImportedLabel:                   "",
				connectivityv1alpha1.ConnectivityRemoteRegistryLabel: r.remoteRegistry.Name,
			},
			Annotations: fs.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "RemoteRegistry",
					UID:        r.remoteRegistry.GetUID(),
					Name:       r.remoteRegistry.GetName(),
				},
			},
		},
		Spec: connectivityv1alpha1.ServiceRecordSpec{
			FQDN:      fs.Fqdn,
			Endpoints: endpoints,
		},
	}

	return serviceRecord
}

func generateServiceRecordName(fqdn string, remoteRegistryName string) string {
	hashedRemoteRegistryName := createHash(remoteRegistryName)
	return fmt.Sprintf("%s-%s", fqdn, hashedRemoteRegistryName)
}

func createHash(s string) string {
	hasher := fnv.New32a()
	// This never returns an error
	_, _ = hasher.Write([]byte(s))
	return fmt.Sprintf("%08x", hasher.Sum32())
}

func (r *registryClient) isDomainAllowed(sr *connectivityv1alpha1.ServiceRecord) bool {
	if len(r.allowedDomains) == 0 {
		return true
	}
	for _, domain := range r.allowedDomains {
		if dns.IsSubDomain(dns.CanonicalName(domain), dns.CanonicalName(sr.Spec.FQDN)) {
			return true
		}
	}
	return false
}

func cloneStringSlice(s []string) []string {
	return append(s[:0:0], s...)
}
