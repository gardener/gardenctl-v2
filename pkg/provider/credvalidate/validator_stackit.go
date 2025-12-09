/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package credvalidate

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/gardener/gardenctl-v2/pkg/provider/common/allowpattern"
	"github.com/gardener/gardenctl-v2/pkg/provider/common/credvalidate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// STACKITValidator implements the common validator interface for STACKIT.
type STACKITValidator struct {
	openstack *OpenStackValidator
	stackit   *credvalidate.BaseValidator
}

var uuidRegex = `^[0-9a-fA-F-]{36}$`

var _ credvalidate.Validator = &STACKITValidator{}

// NewSTACKITValidator creates a new STACKIT validator.
func NewSTACKITValidator(ctx context.Context, allowedPatterns []allowpattern.Pattern) *STACKITValidator {
	return &STACKITValidator{
		openstack: NewOpenStackValidator(ctx, []allowpattern.Pattern{}),
		stackit:   credvalidate.NewBaseValidator(ctx, allowedPatterns),
	}
}

// ValidateSecret validates OpenStack credentials from a Kubernetes secret.
func (v *STACKITValidator) ValidateSecret(secret *corev1.Secret) (map[string]interface{}, error) {
	fields := credvalidate.FlatCoerceBytesToStringsMap(secret.Data)
	registry := map[string]credvalidate.FieldRule{
		"serviceaccount.json": {Required: true, Validator: validateSTACKITServiceaccount, NonSensitive: false},
		"project-id":          {Required: true, Validator: validateSTACKITProjectID, NonSensitive: true},
	}

	validatedValues, err := v.stackit.ValidateWithRegistry(fields, registry, credvalidate.Permissive)
	if err != nil {
		return nil, err
	}

	// The secret is validated against the OpenStack validation in order to get unknown fields removed.
	// But the error is ignored as OpensStack is optional in provider STACKIT and will be removed in the future.
	validatedValuesOpenstack, _ := v.openstack.ValidateSecret(secret)
	maps.Copy(validatedValues, validatedValuesOpenstack)

	return validatedValues, nil
}

func DefaultSTACKITAllowedPatterns() []allowpattern.Pattern {
	return []allowpattern.Pattern{
		{
			Field:      "project-id",
			RegexValue: ptr.To(uuidRegex),
		},
		{
			Field:      "id",
			RegexValue: ptr.To(uuidRegex),
		},
		{
			Field:      "sub",
			RegexValue: ptr.To(uuidRegex),
		},
		{
			Field:      "kid",
			RegexValue: ptr.To(uuidRegex),
		},
		{
			Field:      "keyType",
			RegexValue: ptr.To("^USER_MANAGED|SYSTEM_MANAGED$"),
		},
		{
			Field:      "keyOrigin",
			RegexValue: ptr.To("^USER_PROVIDED|GENERATED$"),
		},
		{
			Field:      "keyAlgorithm",
			RegexValue: ptr.To("^RSA_2048|RSA_4096$"),
		},
		{
			Field:      "iss",
			RegexValue: ptr.To(`^[\w-\.]+@([\w-]+\.)?sa\.stackit\.cloud$`),
		},
	}
}

// GetSTACKITValidationContext returns the validation context for STACKIT patterns.
func GetSTACKITValidationContext() *allowpattern.ValidationContext {
	return &allowpattern.ValidationContext{
		AllowedRegexFields: map[string]bool{
			"project-id":   true,
			"id":           true,
			"sub":          true,
			"kid":          true,
			"keyType":      true,
			"keyOrigin":    true,
			"keyAlgorithm": true,
			"iss":          true,
			"aud":          true,
		},
		StrictHTTPS: true,
		AllowedUserConfigurableFields: map[string]bool{
			"aud": true,
		},
	}
}

func validateSTACKITProjectID(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	return v.ValidateFieldPattern(field, str, all, credvalidate.MatchRegexValuePattern, nonSensitive)
}

func validateSTACKITServiceaccount(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	fields := make(map[string]interface{})

	err = json.Unmarshal([]byte(str), &fields)
	if err != nil {
		return credvalidate.NewFieldError(field, fmt.Sprintf("no valid json %v", err), nil, nonSensitive)
	}

	serviceAccountRegistry := map[string]credvalidate.FieldRule{
		"id":           {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"publicKey":    {Required: false, Validator: validateSTACKITPublicKey, NonSensitive: true},
		"createdAt":    {Required: false, Validator: validateRFC3339Time, NonSensitive: true},
		"validUntil":   {Required: false, Validator: validateRFC3339Time, NonSensitive: true},
		"keyType":      {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"keyOrigin":    {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"keyAlgorithm": {Required: false, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"active":       {Required: false, Validator: validateBool, NonSensitive: true},
		"credentials":  {Required: true, Validator: validateSTACKITServiceaccountCredentials, NonSensitive: false},
	}

	// Validate the nested service account JSON using the base validator
	return v.ValidateNestedFieldsStrict(fields, serviceAccountRegistry)
}

func validateSTACKITServiceaccountCredentials(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	fields, ok := val.(map[string]interface{})
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a map[string]interface", nil, nonSensitive)
	}

	serviceAccountCredentialsRegistry := map[string]credvalidate.FieldRule{
		"kid":        {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"iss":        {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"sub":        {Required: true, Validator: credvalidate.ValidateStringWithPattern(credvalidate.MatchRegexValuePattern), NonSensitive: true},
		"aud":        {Required: true, Validator: validateSTACKITServiceaccountCredentialsAud, NonSensitive: true},
		"privateKey": {Required: true, Validator: validateSTACKITPrivateKey, NonSensitive: true},
	}

	// Validate the nested service account credentials JSON using the base validator
	return v.ValidateNestedFieldsStrict(fields, serviceAccountCredentialsRegistry)
}

// validateSTACKITServiceaccountCredentialsAud validates the aud field.
// 1. validate against MatchRegexValuePattern which can be set by flag or in the config.
// 2. validate against MatchURIPattern which can be set by flag or in the config.
// 3. default to https and stackit.cloud suffix.
func validateSTACKITServiceaccountCredentialsAud(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	err = v.ValidateFieldPattern(field, str, all, credvalidate.MatchRegexValuePattern, nonSensitive)
	if err == nil {
		return nil
	}

	err = v.ValidateFieldPattern(field, str, all, credvalidate.MatchURIPattern, nonSensitive)
	if err == nil {
		return nil
	}

	u, err := url.Parse(str)
	if err != nil {
		return credvalidate.NewFieldError(field, "field cannot be parsed as url", err, nonSensitive)
	}

	if !strings.HasSuffix(u.Host, ".stackit.cloud") {
		return credvalidate.NewFieldError(field, "field is not a valid url", err, nonSensitive)
	}

	if u.Scheme != "https" {
		return credvalidate.NewFieldError(field, "field is using a url without https", err, nonSensitive)
	}

	return nil
}

func validateBool(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	_, ok := val.(bool)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a bool", nil, nonSensitive)
	}

	return nil
}

func validateRFC3339Time(v *credvalidate.BaseValidator, field string, val any, all map[string]any, nonSensitive bool) error {
	str, err := credvalidate.AssertStringWithPrintableCheck(field, val, nonSensitive)
	if err != nil {
		return err
	}

	_, err = time.Parse(time.RFC3339, str)
	if err != nil {
		return credvalidate.NewFieldError(field, "field cannot be parsed as time", err, nonSensitive)
	}

	return nil
}

func validateSTACKITPublicKey(_ *credvalidate.BaseValidator, field string, val any, _ map[string]any, nonSensitive bool) error {
	s, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if !strings.HasPrefix(s, "-----BEGIN ") {
		return credvalidate.NewFieldError(field, "field value must start with a PEM BEGIN line", nil, nonSensitive)
	}

	block, rest := pem.Decode([]byte(s))
	if block == nil {
		return credvalidate.NewFieldError(field, "field value must be a valid PEM-encoded public key", nil, nonSensitive)
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return credvalidate.NewFieldError(field, "field value must contain exactly one PEM block (unexpected data after END line)", nil, nonSensitive)
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return credvalidate.NewFieldError(field, "field value cannot be parsed", err, nonSensitive)
	}

	_, ok = parsed.(*rsa.PublicKey)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be an RSA public key", nil, nonSensitive)
	}

	return nil
}

func validateSTACKITPrivateKey(_ *credvalidate.BaseValidator, field string, val any, _ map[string]any, nonSensitive bool) error {
	s, ok := val.(string)
	if !ok {
		return credvalidate.NewFieldError(field, "field value must be a string", nil, nonSensitive)
	}

	if !strings.HasPrefix(s, "-----BEGIN ") {
		return credvalidate.NewFieldError(field, "field value must start with a PEM BEGIN line", nil, nonSensitive)
	}

	block, rest := pem.Decode([]byte(s))
	if block == nil {
		return credvalidate.NewFieldError(field, "field value must be a valid PEM-encoded public key", nil, nonSensitive)
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return credvalidate.NewFieldError(field, "field value must contain exactly one PEM block (unexpected data after END line)", nil, nonSensitive)
	}

	var err error

	switch block.Type {
	case "PRIVATE KEY":
		_, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return credvalidate.NewFieldError(field, "unknown private key type", err, nonSensitive)
	}

	if err != nil {
		return credvalidate.NewFieldError(field, "field value cannot be parsed", err, nonSensitive)
	}

	return nil
}
