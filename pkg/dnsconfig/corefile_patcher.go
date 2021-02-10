// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dnsconfig

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CorefilePatcher struct {
	Client       client.Client
	Log          logr.Logger
	DomainSuffix string

	Namespace     string
	ConfigMapName string
}

const sectionBegin string = "### BEGIN CROSS CLUSTER CONNECTIVITY"
const sectionEnd string = "### END CROSS CLUSTER CONNECTIVITY"

const xccForwardBlockTemplate string = `%s
%s {
    forward . %s
    reload
}
%s`

var serverBlockRegexp = regexp.MustCompile(`(?s)\s` + sectionBegin + ".*" + sectionEnd)

func (c *CorefilePatcher) AppendStubDomainBlock(forwardingIP string) error {
	var configMap corev1.ConfigMap
	err := c.Client.Get(context.Background(), client.ObjectKey{
		Namespace: c.Namespace,
		Name:      c.ConfigMapName,
	}, &configMap)
	if err != nil {
		return err
	}

	xccBlock := fmt.Sprintf(xccForwardBlockTemplate,
		sectionBegin,
		c.DomainSuffix,
		forwardingIP,
		sectionEnd,
	)

	corefile := configMap.Data["Corefile"]

	if strings.Contains(corefile, xccBlock) {
		c.Log.Info("up to date, skipping modification", "ConfigMap", fmt.Sprintf("%s/%s", c.Namespace, c.ConfigMapName))
		return nil
	}

	corefile = stripXCCBlock(corefile)

	configMap.Data["Corefile"] = fmt.Sprintf("%s\n%s\n", corefile, xccBlock)
	c.Log.Info("updating Corefile", "ConfigMap", fmt.Sprintf("%s/%s", c.Namespace, c.ConfigMapName))

	return c.Client.Update(context.Background(), &configMap)
}

func stripXCCBlock(corefile string) string {
	return strings.TrimSpace(serverBlockRegexp.ReplaceAllString(corefile, ""))
}
