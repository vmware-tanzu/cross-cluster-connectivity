// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicerecorddelete_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServicerecorddelete(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Servicerecorddelete Suite")
}
