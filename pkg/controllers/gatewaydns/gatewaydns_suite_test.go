// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGatewaydns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GatewayDNS Suite")
}
