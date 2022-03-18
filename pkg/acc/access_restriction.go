/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package acc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const minAccessRestrictionMessageWidth = 76

// AccessRestriction is used to define an access restriction
type AccessRestriction struct {
	Key      string                    `yaml:"key,omitempty" json:"key,omitempty"`
	NotifyIf bool                      `yaml:"notifyIf,omitempty" json:"notifyIf,omitempty"`
	Msg      string                    `yaml:"msg,omitempty" json:"msg,omitempty"`
	Options  []AccessRestrictionOption `yaml:"options,omitempty" json:"options,omitempty"`
}

// AccessRestrictionOption is used to define an access restriction option
type AccessRestrictionOption struct {
	Key      string `yaml:"key,omitempty" json:"key,omitempty"`
	NotifyIf bool   `yaml:"notifyIf,omitempty" json:"notifyIf,omitempty"`
	Msg      string `yaml:"msg,omitempty" json:"msg,omitempty"`
}

// AccessRestrictionHandler is a function that should display a single AccessRestrictionMessage to the user.
// The typical implementation of this function looks like this:
//  func(message *AccessRestrictionMessage) { message.Render(os.Stdout) }
type AccessRestrictionHandler func(*AccessRestrictionMessage)
type accessRestrictionHandlerContextKey struct{}

// WithAccessRestrictionHandler returns a copy of parent context to which the given AccessRestrictionHandler function has been added.
func WithAccessRestrictionHandler(ctx context.Context, fn AccessRestrictionHandler) context.Context {
	return context.WithValue(ctx, accessRestrictionHandlerContextKey{}, fn)
}

// AccessRestrictionHandlerFromContext extracts an AccessRestrictionHandler function from the context.
func AccessRestrictionHandlerFromContext(ctx context.Context) AccessRestrictionHandler {
	if val := ctx.Value(accessRestrictionHandlerContextKey{}); val != nil {
		if fn, ok := val.(AccessRestrictionHandler); ok {
			return fn
		}
	}

	return nil
}

// AccessRestrictionMessage collects all messages for an access restriction in order to display them to the user.
type AccessRestrictionMessage struct {
	Header string
	Items  []string
}

func (m *AccessRestrictionMessage) messageWidth() int {
	width := len(m.Header)

	for _, msg := range m.Items {
		l := len(msg) + 2
		if l > width {
			width = l
		}
	}

	if width < minAccessRestrictionMessageWidth {
		width = minAccessRestrictionMessageWidth
	}

	return width
}

// Render writes out an access restriction as a formatted text box that can be displayed to a user in the console.
func (m *AccessRestrictionMessage) Render(w io.Writer) {
	bold := color.New(color.Bold)
	width := m.messageWidth()

	fmt.Fprintf(w, "┌─ %s %s─┐\n", bold.Sprint("Access Restriction"), strings.Repeat("─", width-20))
	fmt.Fprintf(w, "│ %s │\n", m.Header+strings.Repeat(" ", width-len(m.Header)))

	for _, item := range m.Items {
		fmt.Fprintf(w, "│ %s │\n", "* "+item+strings.Repeat(" ", width-len(item)-2))
	}

	fmt.Fprintf(w, "└─%s─┘\n", strings.Repeat("─", width))
}

func (accessRestriction *AccessRestriction) checkAccessRestriction(matchLabels, annotations map[string]string) *AccessRestrictionMessage {
	var matches = func(m map[string]string, key string, val bool) bool {
		if strVal, ok := m[key]; ok {
			if boolVal, err := strconv.ParseBool(strVal); err == nil {
				return boolVal == val
			}
		}

		return false
	}

	if !matches(matchLabels, accessRestriction.Key, accessRestriction.NotifyIf) {
		return nil
	}

	message := &AccessRestrictionMessage{
		Header: accessRestriction.Msg,
	}

	for _, option := range accessRestriction.Options {
		if matches(annotations, option.Key, option.NotifyIf) {
			message.Items = append(message.Items, option.Msg)
		}
	}

	return message
}

// CheckAccessRestrictions returns a list of access restriction messages for a given shoot cluster.
func CheckAccessRestrictions(accessRestrictions []AccessRestriction, shoot *gardencorev1beta1.Shoot) (messages []*AccessRestrictionMessage) {
	seedSelector := shoot.Spec.SeedSelector
	if seedSelector == nil || seedSelector.MatchLabels == nil {
		return
	}

	matchLabels := seedSelector.MatchLabels
	annotations := shoot.GetAnnotations()

	for _, accessRestriction := range accessRestrictions {
		if message := accessRestriction.checkAccessRestriction(matchLabels, annotations); message != nil {
			messages = append(messages, message)
		}
	}

	return messages
}
