/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/gardener/gardenctl-v2/pkg/cmd"
)

func main() {
	gardenctl := cmd.NewDefaultGardenctlCommand()
	gardenctl.DisableAutoGenTag = true

	outDir := os.Getenv("OUT_DIR")
	if outDir == "" {
		outDir = "./docs/help"
	}

	err := doc.GenMarkdownTree(gardenctl, outDir)
	if err != nil {
		log.Fatal(err)
	}
}
