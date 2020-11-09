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
	"time"

	log "github.com/sirupsen/logrus"
	hamletv1alpha1 "github.com/vmware/hamlet/api/types/v1alpha1"
	"github.com/vmware/hamlet/pkg/client"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

var tlsOverride = false

type registryClient struct {
	remoteRegistry *connectivityv1alpha1.RemoteRegistry
	connClientSet  connectivityclientset.Interface

	serviceRecordLister connectivitylisters.ServiceRecordLister

	// sending to this channel forces a redial of the client
	// for when the RemoteRegistry resource changes.
	reDial chan struct{}
	// send to this channel when RemoteRegistry resource is deleted
	stopCh chan struct{}

	namespace string
}

func newRegistryClient(
	remoteRegistry *connectivityv1alpha1.RemoteRegistry,
	connClientSet connectivityclientset.Interface,
	serviceRecordLister connectivitylisters.ServiceRecordLister,
	namespace string,
) *registryClient {
	return &registryClient{
		remoteRegistry:      remoteRegistry,
		connClientSet:       connClientSet,
		serviceRecordLister: serviceRecordLister,
		reDial:              make(chan struct{}),
		stopCh:              make(chan struct{}),
		namespace:           namespace,
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
			case <-streamEndedNotify:
				logger.Info("stream ended")
			case <-r.stopCh:
				logger.Infof("stop signal received")
				cancel()
				return
			}

		}
	}
}

func (r *registryClient) redial(registry *connectivityv1alpha1.RemoteRegistry) {
	// update the remote registry field and send redial signal
	r.remoteRegistry = registry
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

	currentServiceRecord, err := r.serviceRecordLister.ServiceRecords(desiredServiceRecord.Namespace).Get(desiredServiceRecord.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = r.connClientSet.ConnectivityV1alpha1().ServiceRecords(desiredServiceRecord.Namespace).
				Create(desiredServiceRecord)
			return err
		}

		return fmt.Errorf("error getting current FederatedService: %v", err)
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
		return fmt.Errorf("error updating FederatedService: %v", err)
	}

	return nil
}

func (r *registryClient) deleteHamletService(f *hamletv1alpha1.FederatedService) error {
	fs := r.convertToKubernetesServiceRecord(f)

	currentFS, err := r.serviceRecordLister.ServiceRecords(fs.Namespace).Get(fs.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted
			return nil
		}

		return fmt.Errorf("error getting current FederatedService: %v", err)
	}

	err = r.connClientSet.ConnectivityV1alpha1().ServiceRecords(currentFS.Namespace).Delete(currentFS.Name, &metav1.DeleteOptions{})
	return err
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

	serviceRecord := &connectivityv1alpha1.ServiceRecord{
		ObjectMeta: metav1.ObjectMeta{
			// TODO: account for FQDN ending in "."
			// TODO: account for 64 character limit for resource names
			Name:      generateServiceRecordName(fs.Fqdn, r.remoteRegistry.Name),
			Namespace: r.namespace,
			Labels: map[string]string{
				connectivityv1alpha1.ImportedLabel:                   "",
				connectivityv1alpha1.ConnectivityRemoteRegistryLabel: r.remoteRegistry.Name,
			},
			Annotations: fs.Labels,
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
