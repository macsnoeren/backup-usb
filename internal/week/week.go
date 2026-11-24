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

package week

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Schedule is the unit of one backup folder.
type Schedule string

// Schedules.
const (
	ISOWeek Schedule = "isoWeek"
	Day     Schedule = "day"
	Month   Schedule = "month"
	// Insert makes a new backup every time a disk is plugged in: a period
	// is one minute.
	Insert Schedule = "insert"
)

// Schedules lists the known schedules, from often to seldom.
func Schedules() []Schedule { return []Schedule{Insert, Day, ISOWeek, Month} }

// Valid reports whether s is a known schedule.
func (s Schedule) Valid() bool { _, ok := defaultPatterns[s]; return ok }

// defaultPatterns are chosen so that a folder of one schedule never looks
// like a folder of another: the disk can hold both after a change.
var defaultPatterns = map[Schedule]string{
	Insert:  `{year}\{month}-{day}_{hour}{minute}`,
	Day:     `{year}\{month}-{day}`,
	ISOWeek: `{year}\{week}`,
	Month:   `{year}-{month}`,
}

// DefaultPattern is the folder pattern used for s unless another is set.
func DefaultPattern(s Schedule) string { return defaultPatterns[s] }

// Period is one backup period: an ISO week, a day, a month or, for Insert,
// a minute.
type Period struct {
	Schedule Schedule
	// Year is the ISO year for ISOWeek, the calendar year otherwise.
	Year   int
	Week   int // ISOWeek only
	Month  int // Day, Month and Insert
	Day    int // Day and Insert
	Hour   int // Insert only
	Minute int // Insert only
	// Start is the first moment of the period in the local time zone.
	Start time.Time
}

// For returns the period that contains t.
func For(t time.Time, s Schedule) Period {
	loc := t.Location()
	switch s {
	case Day:
		y, m, d := t.Date()
		return Period{Schedule: Day, Year: y, Month: int(m), Day: d, Start: time.Date(y, m, d, 0, 0, 0, 0, loc)}
	case Insert:
		y, m, d := t.Date()
		return Period{Schedule: Insert, Year: y, Month: int(m), Day: d, Hour: t.Hour(), Minute: t.Minute(), Start: t.Truncate(time.Minute)}
	case Month:
		y, m, _ := t.Date()
		return Period{Schedule: Month, Year: y, Month: int(m), Start: time.Date(y, m, 1, 0, 0, 0, 0, loc)}
	default:
		y, w := t.ISOWeek()
		return Period{Schedule: ISOWeek, Year: y, Week: w, Start: isoWeekStart(y, w, loc)}
	}
}

// isoWeekStart returns the Monday that starts ISO week w of ISO year y.
func isoWeekStart(y, w int, loc *time.Location) time.Time {
	// January 4th is always in week 1.
	jan4 := time.Date(y, time.January, 4, 0, 0, 0, 0, loc)
	offset := (int(jan4.Weekday()) + 6) % 7 // days since Monday
	return jan4.AddDate(0, 0, -offset+(w-1)*7)
}

// End returns the first moment after the period.
func (p Period) End() time.Time {
	switch p.Schedule {
	case Insert:
		return p.Start.Add(time.Minute)
	case Day:
		return p.Start.AddDate(0, 0, 1)
	case Month:
		return p.Start.AddDate(0, 1, 0)
	default:
		return p.Start.AddDate(0, 0, 7)
	}
}

// Label is a short human-readable name: "2026-W05", "2026-11-22", "2026-11"
// or "2026-11-22T09:05". It is also the format of the --week flag.
func (p Period) Label() string {
	switch p.Schedule {
	case Insert:
		return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d", p.Year, p.Month, p.Day, p.Hour, p.Minute)
	case Day:
		return fmt.Sprintf("%04d-%02d-%02d", p.Year, p.Month, p.Day)
	case Month:
		return fmt.Sprintf("%04d-%02d", p.Year, p.Month)
	default:
		return fmt.Sprintf("%04d-W%02d", p.Year, p.Week)
	}
}

// Clamp moves t into the period.
func (p Period) Clamp(t time.Time) time.Time {
	if t.Before(p.Start) {
		return p.Start
	}
	if last := p.End().Add(-time.Nanosecond); t.After(last) {
		return last
	}
	return t
}

// placeholder is one {name} in a folder pattern.
type placeholder struct {
	names  []string
	digits int
}

var placeholders = []placeholder{
	{[]string{"year", "jaar"}, 4},
	{[]string{"week"}, 2},
	{[]string{"month", "maand"}, 2},
	{[]string{"day", "dag"}, 2},
	{[]string{"hour", "uur"}, 2},
	{[]string{"minute", "minuut"}, 2},
}

// Pattern is a parsed folder pattern such as "{year}\{week}".
type Pattern struct {
	raw   string
	sched Schedule
	parts []patternPart // one per path component
}

type patternPart struct {
	tokens []token
	re     *regexp.Regexp
	fields []string // canonical placeholder names in order of the groups
}

type token struct {
	literal string
	field   string // canonical name; empty for a literal
	digits  int
}

var literalRe = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// ParsePattern parses and validates a folder pattern for the schedule. Path
// components are separated by "\" or "/". Placeholders are {year} (or
// {jaar}), {week}, {month} ({maand}), {day} ({dag}), {hour} ({uur}) and
// {minute} ({minuut}); other characters may be letters, digits, "-" and "_".
func ParsePattern(pattern string, s Schedule) (*Pattern, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("unknown schedule %q", s)
	}
	norm := strings.ReplaceAll(pattern, `\`, "/")
	if norm == "" || strings.HasPrefix(norm, "/") || strings.HasSuffix(norm, "/") || strings.Contains(norm, "//") {
		return nil, fmt.Errorf("folder pattern %q: empty path component", pattern)
	}
	p := &Pattern{raw: pattern, sched: s}
	seen := map[string]bool{}
	for _, comp := range strings.Split(norm, "/") {
		part, err := parsePart(comp, seen)
		if err != nil {
			return nil, fmt.Errorf("folder pattern %q: %w", pattern, err)
		}
		p.parts = append(p.parts, part)
	}
	need := map[Schedule][]string{
		ISOWeek: {"year", "week"},
		Day:     {"year", "month", "day"},
		Month:   {"year", "month"},
		Insert:  {"year", "month", "day", "hour", "minute"},
	}[s]
	allowed := map[string]bool{}
	for _, n := range need {
		allowed[n] = true
		if !seen[n] {
			return nil, fmt.Errorf("folder pattern %q: schedule %s needs {%s}", pattern, s, n)
		}
	}
	for n := range seen {
		if !allowed[n] {
			return nil, fmt.Errorf("folder pattern %q: {%s} does not fit schedule %s", pattern, n, s)
		}
	}
	return p, nil
}

func parsePart(comp string, seen map[string]bool) (patternPart, error) {
	var part patternPart
	var re strings.Builder
	re.WriteString("^")
	rest := comp
	for rest != "" {
		i := strings.IndexByte(rest, '{')
		lit := rest
		if i >= 0 {
			lit = rest[:i]
		}
		if strings.ContainsAny(lit, "}") || !literalRe.MatchString(lit) {
			return part, fmt.Errorf("invalid characters in %q", comp)
		}
		if lit != "" {
			part.tokens = append(part.tokens, token{literal: lit})
			re.WriteString(regexp.QuoteMeta(lit))
		}
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			return part, fmt.Errorf("unclosed placeholder in %q", comp)
		}
		name := strings.ToLower(rest[i+1 : i+j])
		ph, ok := lookup(name)
		if !ok {
			return part, fmt.Errorf("unknown placeholder {%s}", name)
		}
		canon := ph.names[0]
		if seen[canon] {
			return part, fmt.Errorf("placeholder {%s} used twice", canon)
		}
		seen[canon] = true
		prev := ""
		if n := len(part.tokens); n > 0 {
			prev = part.tokens[n-1].field
		}
		if prev != "" && (prev != "hour" || canon != "minute") { // {hour}{minute} is fine
			return part, fmt.Errorf("placeholders need a separator in %q", comp)
		}
		part.tokens = append(part.tokens, token{field: canon, digits: ph.digits})
		part.fields = append(part.fields, canon)
		fmt.Fprintf(&re, `(\d{%d})`, ph.digits)
		rest = rest[i+j+1:]
	}
	if len(part.tokens) == 0 {
		return part, fmt.Errorf("empty path component")
	}
	re.WriteString("$")
	part.re = regexp.MustCompile(re.String())
	return part, nil
}

func lookup(name string) (placeholder, bool) {
	for _, ph := range placeholders {
		for _, n := range ph.names {
			if n == name {
				return ph, true
			}
		}
	}
	return placeholder{}, false
}

// String returns the pattern as given.
func (p *Pattern) String() string { return p.raw }

// Schedule returns the schedule the pattern was parsed for.
func (p *Pattern) Schedule() Schedule { return p.sched }

// Clashes reports whether a folder of p could be a folder of q, or lie
// inside one, or the other way round. Two patterns of the same schedule are
// the same kind of backup and never clash.
func (p *Pattern) Clashes(q *Pattern) bool {
	if p.sched == q.sched {
		return false
	}
	for i := 0; i < len(p.parts) && i < len(q.parts); i++ {
		if p.parts[i].re.String() != q.parts[i].re.String() {
			return false
		}
	}
	return true
}

// Depth is the number of path components.
func (p *Pattern) Depth() int { return len(p.parts) }

// Folder returns the folder of period per as a slash-separated relative path.
func (p *Pattern) Folder(per Period) string {
	vals := map[string]int{"year": per.Year, "week": per.Week, "month": per.Month, "day": per.Day, "hour": per.Hour, "minute": per.Minute}
	comps := make([]string, len(p.parts))
	for i, part := range p.parts {
		var b strings.Builder
		for _, t := range part.tokens {
			if t.field == "" {
				b.WriteString(t.literal)
			} else {
				fmt.Fprintf(&b, "%0*d", t.digits, vals[t.field])
			}
		}
		comps[i] = b.String()
	}
	return strings.Join(comps, "/")
}

// Match reports whether rel (a relative path with "/" or "\" separators)
// is exactly a folder of this pattern, and returns its period. Values that
// are not a real date or ISO week do not match.
func (p *Pattern) Match(rel string) (Period, bool) {
	comps := strings.Split(strings.ReplaceAll(rel, `\`, "/"), "/")
	if len(comps) != len(p.parts) {
		return Period{}, false
	}
	vals := map[string]int{}
	for i, part := range p.parts {
		m := part.re.FindStringSubmatch(comps[i])
		if m == nil {
			return Period{}, false
		}
		for k, f := range part.fields {
			n, err := strconv.Atoi(m[k+1])
			if err != nil {
				return Period{}, false
			}
			vals[f] = n
		}
	}
	return fromValues(p.sched, Period{Year: vals["year"], Week: vals["week"], Month: vals["month"], Day: vals["day"], Hour: vals["hour"], Minute: vals["minute"]})
}

// fromValues builds a period from the numbers in v and checks that it
// exists.
func fromValues(s Schedule, v Period) (Period, bool) {
	if v.Year < 1970 || v.Year > 9999 {
		return Period{}, false
	}
	switch s {
	case ISOWeek:
		if v.Week < 1 || v.Week > 53 {
			return Period{}, false
		}
		start := isoWeekStart(v.Year, v.Week, time.Local)
		if y, w := start.ISOWeek(); y != v.Year || w != v.Week {
			return Period{}, false
		}
		return Period{Schedule: ISOWeek, Year: v.Year, Week: v.Week, Start: start}, true
	case Day, Insert:
		t := time.Date(v.Year, time.Month(v.Month), v.Day, v.Hour, v.Minute, 0, 0, time.Local)
		if v.Month < 1 || v.Month > 12 || v.Hour > 23 || v.Minute > 59 || t.Day() != v.Day || int(t.Month()) != v.Month {
			return Period{}, false
		}
		if s == Insert {
			// The numbers as written: a time skipped by daylight saving
			// time still names its own folder.
			return Period{Schedule: Insert, Year: v.Year, Month: v.Month, Day: v.Day, Hour: v.Hour, Minute: v.Minute, Start: t}, true
		}
		return For(t, Day), true
	case Month:
		if v.Month < 1 || v.Month > 12 {
			return Period{}, false
		}
		return For(time.Date(v.Year, time.Month(v.Month), 1, 0, 0, 0, 0, time.Local), Month), true
	}
	return Period{}, false
}

var labelRes = map[Schedule]*regexp.Regexp{
	ISOWeek: regexp.MustCompile(`^(\d{4})-?W?(\d{1,2})$`),
	Day:     regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`),
	Month:   regexp.MustCompile(`^(\d{4})-(\d{2})$`),
	Insert:  regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$`),
}

// ParseLabel parses a --week style label: "2026-47" or "2026-W47" for
// weeks, "2026-11-22" for days, "2026-11" for months and "2026-11-22T09:05"
// for insert.
func ParseLabel(label string, s Schedule) (Period, error) {
	re, ok := labelRes[s]
	if !ok {
		return Period{}, fmt.Errorf("unknown schedule %q", s)
	}
	m := re.FindStringSubmatch(label)
	if m == nil {
		return Period{}, fmt.Errorf("invalid period %q", label)
	}
	n := make([]int, len(m))
	for i := 1; i < len(m); i++ {
		n[i], _ = strconv.Atoi(m[i])
	}
	v := Period{Year: n[1]}
	switch s {
	case ISOWeek:
		v.Week = n[2]
	case Month:
		v.Month = n[2]
	case Day:
		v.Month, v.Day = n[2], n[3]
	case Insert:
		v.Month, v.Day, v.Hour, v.Minute = n[2], n[3], n[4], n[5]
	}
	per, ok := fromValues(s, v)
	if !ok {
		return Period{}, fmt.Errorf("period %q does not exist", label)
	}
	return per, nil
}
