/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	"github.com/gardener/gardenctl-v2/internal/util"
	commonTarget "github.com/gardener/gardenctl-v2/pkg/cmd/common/target"
)

func validArgsFunction(f util.Factory, o *Options, args []string, toComplete string) ([]string, error) {
	if len(args) == 0 {
		return []string{
			string(commonTarget.TargetKindGarden),
			string(commonTarget.TargetKindProject),
			string(commonTarget.TargetKindSeed),
			string(commonTarget.TargetKindShoot),
		}, nil
	}

	return []string{}, nil
}
