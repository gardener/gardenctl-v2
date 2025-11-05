/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"github.com/gardener/gardenctl-v2/pkg/cmd/base"
)

var (
	ValidGardenArgsFunctionWrapper = validGardenArgsFunctionWrapper
	ValidatePatterns               = validatePatterns
)

type CobraValidArgsFunction cobraValidArgsFunction

type ViewOptions struct {
	viewOptions
}

func NewViewOptions() *ViewOptions {
	return &ViewOptions{
		viewOptions: viewOptions{
			Options: base.Options{},
		},
	}
}

type SetGardenOptions struct {
	setGardenOptions
}

func NewSetGardenOptions() *SetGardenOptions {
	return &SetGardenOptions{
		setGardenOptions: setGardenOptions{
			Options: base.Options{},
		},
	}
}

type DeleteGardenOptions struct {
	deleteGardenOptions
}

func NewDeleteGardenOptions() *DeleteGardenOptions {
	return &DeleteGardenOptions{
		deleteGardenOptions: deleteGardenOptions{
			Options: base.Options{},
		},
	}
}

type SetOpenStackAuthURLOptions struct {
	setOpenStackAuthURLOptions
}

func NewSetOpenStackAuthURLOptions() *SetOpenStackAuthURLOptions {
	return &SetOpenStackAuthURLOptions{
		setOpenStackAuthURLOptions: setOpenStackAuthURLOptions{
			Options: base.Options{},
		},
	}
}
