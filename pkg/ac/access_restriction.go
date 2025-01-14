/*
SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package ac

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// AccessRestriction is used to define an access restriction.
type AccessRestriction struct {
	// Key is the identifier of an access restriction
	Key string `json:"key,omitempty"`
	// Msg is the notification text that is sent
	Msg string `json:"msg,omitempty"`
	// Options is a list of access restriction options
	Options []AccessRestrictionOption `json:"options,omitempty"`
}

// AccessRestrictionOption is used to define an access restriction option.
type AccessRestrictionOption struct {
	// Key is the identifier of an access restriction option
	Key string `json:"key,omitempty"`
	// NotifyIf controls which value the annotation must have for a notification to be sent
	NotifyIf bool `json:"notifyIf,omitempty"`
	// Msg is the notification text that is sent
	Msg string `json:"msg,omitempty"`
}

// AccessRestrictionHandler is a function that should display a single AccessRestrictionMessage to the user.
// The typical implementation of this function looks like this:
//
//	func(messages AccessRestrictionMessages) { message.Render(os.Stdout) }
type (
	AccessRestrictionHandler           func(AccessRestrictionMessages) bool
	accessRestrictionHandlerContextKey struct{}
)

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

// NewAccessRestrictionHandler create an access restriction handler function.
func NewAccessRestrictionHandler(r io.Reader, w io.Writer, askForConfirmation bool) AccessRestrictionHandler {
	return func(messages AccessRestrictionMessages) bool {
		if len(messages) == 0 {
			return true
		}

		messages.Render(w)

		if !askForConfirmation {
			return true
		}

		return messages.Confirm(r, w)
	}
}

func (m *AccessRestrictionMessage) messageWidth() int {
	width := 0

	for _, text := range m.Items {
		for _, line := range strings.Split(text, "\n") {
			width = max(width, len(line))
		}
	}

	width += 2

	for _, line := range strings.Split(m.Header, "\n") {
		width = max(width, len(line))
	}

	return width
}

func (ar *AccessRestriction) checkAccessRestriction(all []gardencorev1beta1.AccessRestrictionWithOptions) *AccessRestrictionMessage {
	matches := func(m map[string]string, key string, val bool) bool {
		rawVal, ok := m[key]
		if !ok {
			return false
		}

		boolVal, err := strconv.ParseBool(rawVal)
		if err != nil {
			// If parsing fails, skip it
			return false
		}

		return boolVal == val
	}

	effectiveKey := mapLegacyKey(ar.Key)

	var match *gardencorev1beta1.AccessRestrictionWithOptions

	for _, item := range all {
		if item.Name == effectiveKey {
			match = &item
			break // only one match is possible, so we can break early
		}
	}

	if match == nil {
		return nil
	}

	message := &AccessRestrictionMessage{
		Header: ar.Msg,
	}

	for _, option := range ar.Options {
		if matches(match.Options, option.Key, option.NotifyIf) {
			message.Items = append(message.Items, option.Msg)
		}
	}

	return message
}

// mapLegacyKey maps the legacy seed.gardener.cloud/eu-access access restriction key to the new one, if applicable.
func mapLegacyKey(key string) string {
	if key == "seed.gardener.cloud/eu-access" {
		return "eu-access-only"
	}

	return key
}

// CheckAccessRestrictions returns a list of access restriction messages for a given shoot cluster.
func CheckAccessRestrictions(accessRestrictions []AccessRestriction, shoot *gardencorev1beta1.Shoot) (messages AccessRestrictionMessages) {
	if len(shoot.Spec.AccessRestrictions) == 0 {
		return
	}

	for _, accessRestriction := range accessRestrictions {
		if message := accessRestriction.checkAccessRestriction(shoot.Spec.AccessRestrictions); message != nil {
			messages = append(messages, message)
		}
	}

	return messages
}

// AccessRestrictionMessage collects all messages for an access restriction in order to display them to the user.
type AccessRestrictionMessage struct {
	Header string
	Items  []string
}

// AccessRestrictionMessages is a list of access restriction messages.
type AccessRestrictionMessages []*AccessRestrictionMessage

var _ fmt.Stringer = &AccessRestrictionMessages{}

type pos int

const (
	header pos = iota
	body
	footer
)

func (p pos) start() string {
	switch p {
	case header:
		return "┌─"
	case footer:
		return "└─"
	default:
		return "│ "
	}
}

func (p pos) end() string {
	switch p {
	case header:
		return "─┐"
	case footer:
		return "─┘"
	default:
		return " │"
	}
}

func (p pos) paddEnd(text string, width int) string {
	switch p {
	case header, footer:
		return fmt.Sprintf("%s%s", text, strings.Repeat("─", width-len(text)))
	default:
		return fmt.Sprintf("%-*s", width, text)
	}
}

func (p pos) print(text string, width int) string {
	var results []string

	hasPrefix := strings.HasPrefix(text, "* ")
	for i, line := range strings.Split(text, "\n") {
		if hasPrefix && i > 0 {
			line = "  " + line
		}

		results = append(results, p.start()+p.paddEnd(line, width)+p.end())
	}

	return strings.Join(results, "\n")
}

func (messages AccessRestrictionMessages) String() string {
	b := &bytes.Buffer{}
	messages.Render(b)

	return b.String()
}

// Render displays the access restriction messages.
func (messages AccessRestrictionMessages) Render(w io.Writer) {
	title := " Access Restriction"
	if len(messages) > 1 {
		title += "s"
	}

	title += " "
	width := len(title)

	for _, m := range messages {
		mw := m.messageWidth()
		if mw > width {
			width = mw
		}
	}

	fmt.Fprintln(w, header.print(title, width))

	for _, m := range messages {
		fmt.Fprintln(w, body.print(m.Header, width))

		for _, item := range m.Items {
			fmt.Fprintln(w, body.print("* "+item, width))
		}
	}

	fmt.Fprintln(w, footer.print("", width))
}

// Confirm  asks for confirmation to continue.
func (messages AccessRestrictionMessages) Confirm(r io.Reader, w io.Writer) bool {
	reader := bufio.NewReader(r)

	for {
		fmt.Fprint(w, "Do you want to continue? [y/N]: ")

		str, _ := reader.ReadString('\n')

		str = strings.TrimSpace(str)
		str = strings.ToLower(str)

		switch str {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}
