/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package drop

import (
	"github.com/gardener/gardenctl-v2/internal/util"
)

func validArgsFunction(f util.Factory, o *Options, args []string, toComplete string) ([]string, error) {
	if len(args) == 0 {
		return []string{
			string(TargetKindGarden),
			string(TargetKindProject),
			string(TargetKindSeed),
			string(TargetKindShoot),
		}, nil
	}
	return []string{}, nil
}
