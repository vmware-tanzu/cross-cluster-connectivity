// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"regexp"
	"strings"

	"github.com/miekg/dns"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func (r *RemoteRegistry) ValidateDomains() error {
	var allErrs field.ErrorList
	for i, domain := range r.Spec.AllowedDomains {
		if err := validateDomainName(i, domain); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: SchemeGroupVersion.Group, Kind: "RemoteRegistry"},
		r.Name,
		allErrs,
	)
}

var (
	validCharsRegex = regexp.MustCompile(`\A[a-zA-Z0-9\-\.]+\z`)
	allNumbersRegex = regexp.MustCompile(`\A[0-9]+\z`)
)

const (
	invalidDomain       = "Invalid domain. For example, labels in the domain name must have between 1 and 63 characters. Check RFC1035 for more details."
	invalidChars        = "Invalid characters. Domain names must contain only alphanumeric characters, or dots (.) and hyphens (-)."
	invlalidLabelHyphen = "Invalid use of hyphen in label. Labels in a domain name must not start or end with `-`."
	invalidTLD          = "Invalid TLD. All-numeric TLDs are invalid."
)

func validateDomainName(idx int, domain string) *field.Error {
	fieldPath := field.NewPath("spec").Child("allowedDomains").Index(idx)

	if len(domain) == 0 {
		return field.Required(fieldPath, invalidDomain)
	}

	if !validCharsRegex.MatchString(domain) {
		return field.Invalid(fieldPath, domain, invalidChars)
	}
	if _, ok := dns.IsDomainName(domain); !ok {
		if len(domain) > 255 {
			return field.TooLong(fieldPath, domain, 255)
		}
		return field.Invalid(fieldPath, domain, invalidDomain)
	}
	labels := dns.SplitDomainName(domain)
	for _, label := range labels {
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return field.Invalid(fieldPath, domain, invlalidLabelHyphen)
		}
	}

	if len(labels) > 0 && allNumbersRegex.MatchString(labels[len(labels)-1]) {
		return field.Invalid(fieldPath, domain, invalidTLD)
	}
	return nil
}
