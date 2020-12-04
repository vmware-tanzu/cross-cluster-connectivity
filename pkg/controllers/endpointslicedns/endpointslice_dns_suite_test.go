// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EndpointSlice DNS Suite")
}
