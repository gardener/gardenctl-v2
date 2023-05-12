/*
SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package plugins

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"plugin"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

const (
	gardenHomeFolder = ".garden"
	pluginsFolder    = "plugins"
)

func LoadPlugins(pluginPath string) ([]*cobra.Command, error) {
	plugins := []string{}

	var b plugin.Symbol

	var cmds []*cobra.Command

	files, err := os.ReadDir(pluginPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		filePath := filepath.Join(pluginPath, file.Name())
		ext := path.Ext(filePath)

		if ext == ".so" {
			plugins = append(plugins, filePath)
		}
	}

	if len(plugins) == 0 {
		return nil, nil
	}

	for _, item := range plugins {
		p, err := plugin.Open(item)
		if err != nil {
			return nil, fmt.Errorf("plugin open error or invalid file %w", err)
		}

		b, err = p.Lookup("NewCmd")
		if err != nil {
			return nil, fmt.Errorf("plugin cmd name not found %w", err)
		}

		_, ok := b.(**cobra.Command)
		if !ok {
			return nil, fmt.Errorf("type assertion error")
		}

		cmds = append(cmds, *b.(**cobra.Command))
	}

	return cmds, nil
}

func Load() []*cobra.Command {
	var cmds []*cobra.Command

	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Plugins Error:", err)
	}

	pluginsPath := filepath.Join(home, gardenHomeFolder, pluginsFolder)

	if _, err := os.Stat(pluginsPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
	} else {
		cmds, err = LoadPlugins(pluginsPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Plugins Error:", err)
		}
	}

	return cmds
}
