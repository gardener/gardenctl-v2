/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ssh

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd/api"

	gc "github.com/gardener/gardenctl-v2/internal/gardenclient"
	"github.com/gardener/gardenctl-v2/internal/util"
	"github.com/gardener/gardenctl-v2/pkg/target"
)

var getCurrentUserOriginal = getCurrentUser

func SetGetCurrentUser(f func(ctx context.Context, gardenClient gc.Client, authInfo *api.AuthInfo) (string, error)) {
	getCurrentUser = f
}

func GetGetCurrentUser() func(ctx context.Context, gardenClient gc.Client, authInfo *api.AuthInfo) (string, error) {
	return getCurrentUser
}

func ResetGetCurrentUser() {
	getCurrentUser = getCurrentUserOriginal
}

func GetGetAuthInfo() func(ctx context.Context, manager target.Manager) (*api.AuthInfo, error) {
	return getAuthInfo
}

func GetGetBastionNameCompletions(o *SSHPatchOptions) func(f util.Factory, cmd *cobra.Command, toComplete string) ([]string, error) {
	return o.getBastionNameCompletions
}

var timeNowOriginal = timeNow

func SetTimeNow(f func() time.Time) {
	timeNow = f
}

func GetTimeNow() func() time.Time {
	return timeNow
}

func ResetTimeNow() {
	timeNow = timeNowOriginal
}
