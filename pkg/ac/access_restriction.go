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
	// NotifyIf controls which value the annotation must have for a notification to be sent
	NotifyIf bool `json:"notifyIf,omitempty"`
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

func max(x, y int) int {
	if y > x {
		return y
	}

	return x
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

func (accessRestriction *AccessRestriction) checkAccessRestriction(matchLabels, annotations map[string]string) *AccessRestrictionMessage {
	matches := func(m map[string]string, key string, val bool) bool {
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
func CheckAccessRestrictions(accessRestrictions []AccessRestriction, shoot *gardencorev1beta1.Shoot) (messages AccessRestrictionMessages) {
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
