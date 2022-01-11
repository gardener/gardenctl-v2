/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	sprigv3 "github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardenctl-v2/internal/util"
)

//go:embed templates
var fsys embed.FS

//go:generate mockgen -destination=./mocks/mock_template.go -package=mocks github.com/gardener/gardenctl-v2/pkg/cmd/env Template

// Template provides an abstraction for go templates
type Template interface {
	// ParseFiles parses the named files and associates the resulting templates with t.
	ParseFiles(filenames ...string) error
	// ExecuteTemplate applies the template associated with t for the given shell
	// to the specified data object and writes the output to wr.
	ExecuteTemplate(wr io.Writer, shell string, data interface{}) error
}

type templateImpl struct {
	delegate *template.Template
}

var _ Template = &templateImpl{}

func newTemplate(filenames ...string) Template {
	return newTemplateImpl(filenames...)
}

func newTemplateImpl(filenames ...string) *templateImpl {
	t := &templateImpl{
		delegate: template.
			New("base").
			Funcs(sprigv3.TxtFuncMap()).
			Funcs(template.FuncMap{"shellEscape": util.ShellEscape}),
	}

	if len(filenames) > 0 {
		utilruntime.Must(t.ParseFiles(filenames...))
	}

	return t
}

// ParseFiles parses the file located at path and associates the resulting templates with t.
func (t *templateImpl) ParseFiles(filenames ...string) error {
	for _, filename := range filenames {
		if err := parseFile(fsys, t.delegate, filename); err != nil {
			return err
		}
	}

	return nil
}

// ExecuteTemplate applies the template associated with t for the given shell
// to the specified data object and writes the output to wr.
func (t *templateImpl) ExecuteTemplate(wr io.Writer, shell string, data interface{}) error {
	return t.delegate.ExecuteTemplate(wr, shell, data)
}

// Delegate returns the wrapped go template
func (t *templateImpl) Delegate() *template.Template {
	return t.delegate
}

func parseFile(fsys fs.FS, t *template.Template, filename string) error {
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	pattern := filepath.Join("templates", name+".tmpl")
	embedExist := false

	list, err := fs.Glob(fsys, pattern)
	if err != nil {
		return err
	}

	if len(list) > 0 {
		embedExist = true

		_, err := t.ParseFS(fsys, pattern)
		if err != nil {
			return fmt.Errorf("parsing embedded template %q failed: %w", name, err)
		}
	}

	if filepath.IsAbs(filename) {
		_, err := t.ParseFiles(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if !embedExist {
					return fmt.Errorf("template %q does not exist: %w", name, err)
				}
			} else {
				return fmt.Errorf("parsing template %q failed: %w", name, err)
			}
		}
	}

	return nil
}
