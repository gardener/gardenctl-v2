/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"log"

	"github.com/spf13/cobra/doc"

	"github.com/gardener/gardenctl-v2/pkg/cmd"
)

func main() {
	gardenctl := cmd.NewDefaultGardenctlCommand()
	err := doc.GenMarkdownTree(gardenctl, "./docs/help")

	if err != nil {
		log.Fatal(err)
	}
}
