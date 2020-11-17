// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orphandeleter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOrphanDeleter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OrphanDeleter Suite")
}
