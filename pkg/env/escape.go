/*
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/

package env

import (
	"fmt"
	"strings"

	"github.com/gardener/gardenctl-v2/internal/util"
)

// All runes that PowerShell treats as a single-quote token.
const (
	// ' ascii apostrophe.
	quoteAsciiApostrophe rune = '\u0027'
	// ‘ left single quotation mark.
	quoteSingleLeft rune = '\u2018'
	// ’ right single quotation mark.
	quoteSingleRight rune = '\u2019'
	// ‚ single low-9 quotation mark.
	quoteSingleLow9 rune = '\u201A'
	// ‛ single high-reversed-9 quotation mark.
	quoteSingleHighRev9 rune = '\u201B'
)

// shellEscapePowerShell returns a single-quoted literal for PowerShell.
// It handles multiple Unicode quote types (apostrophe, left/right quotation marks, etc.)
// by doubling them for escaping. Non-printable characters are removed.
func shellEscapePowerShell(values ...interface{}) string {
	singleQuoteRunes := []rune{
		quoteAsciiApostrophe,
		quoteSingleLeft,
		quoteSingleRight,
		quoteSingleLow9,
		quoteSingleHighRev9,
	}

	out := make([]string, 0, len(values))

	for _, v := range values {
		if v == nil {
			continue
		}

		s := fmt.Sprintf("%v", v)
		s = util.StripUnsafe(s)

		// Double every quote-type rune directly.
		for _, qr := range singleQuoteRunes {
			rep := strings.Repeat(string(qr), 2) // e.g., "'" -> "''"
			s = strings.ReplaceAll(s, string(qr), rep)
		}

		s = "'" + s + "'"
		out = append(out, s)
	}

	return strings.Join(out, " ")
}

// shellEscapeFish returns a shell-escaped version of the given string for Fish shell.
// Non-printable characters are removed.
func shellEscapeFish(values ...interface{}) string {
	out := make([]string, 0, len(values))

	for _, v := range values {
		if v == nil {
			continue
		}

		s := fmt.Sprintf("%v", v)
		s = util.StripUnsafe(s)
		// Escape backslashes first
		s = strings.ReplaceAll(s, "\\", "\\\\")
		// Then escape single quotes: end quote, add escaped quote, resume quote
		s = strings.ReplaceAll(s, "'", "'\\''")
		s = "'" + s + "'"
		out = append(out, s)
	}

	return strings.Join(out, " ")
}

// shellEscapePOSIX quotes values for POSIX shells (bash, zsh).
// Non-printable characters are removed.
func shellEscapePOSIX(values ...interface{}) string {
	out := make([]string, 0, len(values))

	for _, v := range values {
		if v == nil {
			continue
		}

		s := fmt.Sprintf("%v", v)
		s = util.StripUnsafe(s)
		s = strings.ReplaceAll(s, "'", "'\"'\"'")
		s = "'" + s + "'"
		out = append(out, s)
	}

	return strings.Join(out, " ")
}

// ShellEscapeFor returns a shell-aware escape function suitable for use in templates or callers.
// It implements POSIX-like escaping for bash/zsh, a fish-safe variant, and a
// PowerShell-safe escaper. All escapers also strip non-printable characters.
func ShellEscapeFor(shell string) (func(values ...interface{}) string, error) {
	switch Shell(shell) {
	case powershell:
		return shellEscapePowerShell, nil
	case fish:
		return shellEscapeFish, nil
	case bash, zsh:
		return shellEscapePOSIX, nil
	default:
		return nil, fmt.Errorf("invalid shell given, must be one of %v", ValidShells())
	}
}
