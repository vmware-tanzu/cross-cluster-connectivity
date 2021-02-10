// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dnsconfig_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDNSConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DNS Config Suite")
}
