/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package cmd

const (
	EnvGardenHomeDir = envGardenHomeDir
	EnvSessionID     = envPrefix + "_SESSION_ID"
	ConfigName       = configName
)

var (
	ShootFlagCompletionFunc   = shootFlagCompletionFunc
	SeedFlagCompletionFunc    = seedFlagCompletionFunc
	ProjectFlagCompletionFunc = projectFlagCompletionFunc
	GardenFlagCompletionFunc  = gardenFlagCompletionFunc
	CompletionWrapper         = completionWrapper
)
