// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validation of the domain names provided in the AllowedDomains list adheres
// to the following rules:
// 1. A fully qualified domain name, cannot exceed 255 characters. (See
//    RFC-1035 and RFC-2181)
// 2. Labels in the fully qualified domain name cannot exceed 63 characters.
//    Labels also must not be empty.  (See RFC-1035 and RFC-2181)
// 3. Domain names containing characters that are not alphanumeric, a period,
//    or a hyphen are not valid. This convention was specified in RFC-952, but
//    it is not a strict requirement of FQDNs in general. Thus, any FQDNs
//    containing special unicode characters should be converted to a Punycode
//    representation before adding it to the AllowedDomains list.
// 4. Labels in the domain name cannot start or end with hyphens. (See RFC-952
//    and RFC-1123).
// 5. All-numeric TLDs are not not valid. (See RFC3696).
var _ = Describe("RemoteRegistry Validation", func() {

	DescribeTable("ValidateDomains",
		func(domain string, expectedErrMatcher interface{}) {
			remoteRegistry := &connectivityv1alpha1.RemoteRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-remote-registry",
					Namespace: "cross-cluster-connectivity",
					UID:       "remote-registry-uid",
				},
				Spec: connectivityv1alpha1.RemoteRegistrySpec{
					Address:        "127.0.0.1:8000",
					AllowedDomains: []string{domain},
				},
			}
			if expectedErrMatcher == nil {
				Expect(remoteRegistry.ValidateDomains()).To(Succeed())
				return
			}
			Expect(remoteRegistry.ValidateDomains()).To(MatchError(expectedErrMatcher))
		},
		Entry("domain with just the root zone is valid", ".", nil),
		Entry("a domain with just one label is valid", "tld", nil),
		Entry("a fully qualified domain name with just one label is valid", "tld.", nil),
		Entry("a standard domain is valid", "example.com", nil),
		Entry("a domain with an all-numeric label (not TLD) is valid", "123.local", nil),

		Entry("a domain that is longer than 255 characters is invalid",
			fmt.Sprintf("%s.%s.%s.%s.%s",
				strings.Repeat("a", 60),
				strings.Repeat("b", 60),
				strings.Repeat("c", 60),
				strings.Repeat("d", 60),
				strings.Repeat("e", 12),
			),
			ContainSubstring("Too long"),
		),
		Entry("a domain that is 254 characters is valid",
			fmt.Sprintf("%s.%s.%s.%s.%s",
				strings.Repeat("a", 60),
				strings.Repeat("b", 60),
				strings.Repeat("c", 60),
				strings.Repeat("d", 60),
				strings.Repeat("e", 11),
			),
			nil,
		),
		Entry("a domain that is 254 characters and also a trailing dot is valid",
			fmt.Sprintf("%s.%s.%s.%s.%s.",
				strings.Repeat("a", 60),
				strings.Repeat("b", 60),
				strings.Repeat("c", 60),
				strings.Repeat("d", 60),
				strings.Repeat("e", 11),
			),
			nil,
		),

		Entry("an empty domain is invalid", "", ContainSubstring("Invalid domain")),
		Entry("a domain with a label that is longer than 63 characters is invalid",
			fmt.Sprintf("%s.example.com", strings.Repeat("a", 64)),
			ContainSubstring("Invalid domain"),
		),
		Entry("a domain with a 0 length label is invalid", "example..com", ContainSubstring("Invalid domain")),
		Entry("a domain with a label that is exactly 63 characters is valid",
			fmt.Sprintf("%s.example.com", strings.Repeat("a", 63)),
			nil,
		),

		Entry("a domain with the scheme is invalid",
			"http://example.com",
			ContainSubstring("Invalid characters"),
		),
		Entry("a domain with the path is invalid",
			"example.com/path",
			ContainSubstring("Invalid characters"),
		),
		Entry("a domain with whitespace is invalid",
			"example domain.com",
			ContainSubstring("Invalid characters"),
		),
		Entry("a domain with backslashes is invalid",
			"\\123example.com",
			ContainSubstring("Invalid characters"),
		),
		Entry("a domain with unconverted unicode characters is invalid",
			"你好.com",
			ContainSubstring("Invalid characters"),
		),
		Entry("a domain with a punycode representation is valid",
			"xn--6qq79v.com",
			nil,
		),

		Entry("a domain with a label beginning with a hyphen is invalid",
			"example.-domain.com",
			ContainSubstring("Invalid use of hyphen"),
		),
		Entry("a domain with a label ending with a hyphen is invalid",
			"example-.domain.com",
			ContainSubstring("Invalid use of hyphen"),
		),
		Entry("a domain with a label beginning and ending with a hyphen is invalid",
			"example.-domain-.com",
			ContainSubstring("Invalid use of hyphen"),
		),
		Entry("a domain with a hyphen in the middle of a label is valid",
			"example-domain.com",
			nil,
		),

		Entry("a domain with one all-numeric label is invalid",
			"123",
			ContainSubstring("Invalid TLD"),
		),
		Entry("a domain with an all-numeric TLD is invalid",
			"example.123",
			ContainSubstring("Invalid TLD"),
		),
		Entry("a fully qualified domain name with an all-numeric TLD is invalid",
			"example.123.",
			ContainSubstring("Invalid TLD"),
		),
	)

	When("there are multiple domains that fail validation", func() {
		var remoteRegistry *connectivityv1alpha1.RemoteRegistry
		BeforeEach(func() {
			remoteRegistry = &connectivityv1alpha1.RemoteRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-remote-registry",
					Namespace: "cross-cluster-connectivity",
					UID:       "remote-registry-uid",
				},
				Spec: connectivityv1alpha1.RemoteRegistrySpec{
					Address: "127.0.0.1:8000",
					AllowedDomains: []string{
						"example domain.com",
						"example.-domain.com",
					},
				},
			}
		})

		It("returns errors for each domain that failed validation", func() {
			Expect(remoteRegistry.ValidateDomains()).To(MatchError(And(
				ContainSubstring("Invalid characters"),
				ContainSubstring("Invalid use of hyphen"),
			)))
		})
	})
})
