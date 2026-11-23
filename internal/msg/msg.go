// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

// Package msg holds the translation table for all user-facing texts.
// Code, comments and log messages are English; everything a user reads goes
// through T.
package msg

import (
	"fmt"
	"sort"
)

// Lang is a supported language.
type Lang string

// Supported languages.
const (
	NL Lang = "nl"
	EN Lang = "en"
)

// Langs lists the supported languages.
var Langs = []Lang{EN, NL}

// Default is the default language.
const Default = EN

// T returns the text for key in lang, formatted with args. Missing
// translations fall back to English, then to the key itself.
func T(lang Lang, key string, args ...any) string {
	e, ok := table[key]
	if !ok {
		return key
	}
	s := e[lang]
	if s == "" {
		s = e[EN]
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Keys returns all keys in the table, sorted.
func Keys() []string {
	out := make([]string, 0, len(table))
	for k := range table {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Has reports whether key is in the table.
func Has(key string) bool {
	_, ok := table[key]
	return ok
}

// Printer formats texts in one language.
type Printer struct{ Lang Lang }

// T formats key in the printer's language.
func (p Printer) T(key string, args ...any) string { return T(p.Lang, key, args...) }

type entry map[Lang]string
