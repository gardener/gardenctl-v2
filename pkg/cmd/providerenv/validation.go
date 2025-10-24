/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package providerenv

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// reservedProviderTypes contains provider types that are reserved and cannot be used.
var reservedProviderTypes = map[string]bool{
	"kubernetes": true, // Reserved for kubectl env command
}

// ValidateProviderType validates that the provider type is safe to use in file paths.
func ValidateProviderType(providerType string) error {
	if len(providerType) == 0 {
		return fmt.Errorf("provider type cannot be empty")
	}

	if reservedProviderTypes[providerType] {
		return fmt.Errorf("provider type %q is reserved and cannot be used", providerType)
	}

	if reasons := validation.IsDNS1123Label(providerType); len(reasons) > 0 {
		return fmt.Errorf("invalid provider type %q: %s", providerType, strings.Join(reasons, "; "))
	}

	return nil
}

// ValidateCLIName validates that a CLI name is safe to use in file paths.
func ValidateCLIName(cli string) error {
	if len(cli) == 0 {
		return fmt.Errorf("cli name cannot be empty")
	}

	if reasons := validation.IsDNS1123Label(cli); len(reasons) > 0 {
		return fmt.Errorf("invalid cli name %q: %s", cli, strings.Join(reasons, "; "))
	}

	return nil
}
