/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package sshpatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gardener/gardenctl-v2/internal/util"
)

func GetBastionNameCompletions(f util.Factory, cmd *cobra.Command, prefix string) ([]string, error) {
	ctx, cancel := context.WithTimeout(f.Context(), 30*time.Second)
	defer cancel()

	manager, err := f.Manager()
	if err != nil {
		return nil, fmt.Errorf("failed to get manager: %w", err)
	}

	userBastionLister, err := newUserBastionListPatcher(manager)
	if err != nil {
		return nil, fmt.Errorf("could not create bastion lister: %w", err)
	}

	bastions, err := userBastionLister.List(ctx)
	if err != nil {
		return nil, err
	}

	var completions []string

	clock := f.Clock()

	for _, b := range bastions {
		if strings.HasPrefix(b.Name, prefix) {
			age := clock.Now().Sub(b.CreationTimestamp.Time).Round(time.Second).String()

			completion := fmt.Sprintf("%s\t created %s ago targeting shoot \"%s/%s\"", b.Name, age, b.Namespace, b.Spec.ShootRef.Name)
			completions = append(completions, completion)
		}
	}

	return completions, nil
}
