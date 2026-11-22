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
