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
)

// Valid reports whether s is a known schedule.
func (s Schedule) Valid() bool { return s == ISOWeek || s == Day || s == Month }

// Period is one backup period: an ISO week, a day or a month.
type Period struct {
	Schedule Schedule
	// Year is the ISO year for ISOWeek, the calendar year otherwise.
	Year  int
	Week  int // ISOWeek only
	Month int // Day and Month
	Day   int // Day only
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
	case Day:
		return p.Start.AddDate(0, 0, 1)
	case Month:
		return p.Start.AddDate(0, 1, 0)
	default:
		return p.Start.AddDate(0, 0, 7)
	}
}

// Label is a short human-readable name: "2026-W05", "2026-11-22" or
// "2026-11". It is also the format of the --week flag.
func (p Period) Label() string {
	switch p.Schedule {
	case Day:
		return fmt.Sprintf("%04d-%02d-%02d", p.Year, p.Month, p.Day)
	case Month:
		return fmt.Sprintf("%04d-%02d", p.Year, p.Month)
	default:
		return fmt.Sprintf("%04d-W%02d", p.Year, p.Week)
	}
}

// Number is the calendar number used for odd/even rotation: the ISO week,
// the day of the month or the month. Like weeks 53 and 1, day 31 and day 1
// are both odd.
func (p Period) Number() int {
	switch p.Schedule {
	case Day:
		return p.Day
	case Month:
		return p.Month
	default:
		return p.Week
	}
}

// Ordinal is a continuous counter of periods, used for round-robin rotation.
func (p Period) Ordinal() int64 {
	switch p.Schedule {
	case Day:
		return daysSinceEpoch(p.Year, time.Month(p.Month), p.Day)
	case Month:
		return int64(p.Year)*12 + int64(p.Month) - 1
	default:
		s := isoWeekStart(p.Year, p.Week, time.UTC)
		// 1970-01-05 was a Monday.
		return (daysSinceEpoch(s.Year(), s.Month(), s.Day()) - 4) / 7
	}
}

func daysSinceEpoch(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
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
}

// Pattern is a parsed folder pattern such as "{year}\{week}".
type Pattern struct {
	raw   string
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
// {jaar}), {week}, {month} ({maand}) and {day} ({dag}); other characters may
// be letters, digits, "-" and "_".
func ParsePattern(pattern string, s Schedule) (*Pattern, error) {
	if !s.Valid() {
		return nil, fmt.Errorf("unknown schedule %q", s)
	}
	norm := strings.ReplaceAll(pattern, `\`, "/")
	if norm == "" || strings.HasPrefix(norm, "/") || strings.HasSuffix(norm, "/") || strings.Contains(norm, "//") {
		return nil, fmt.Errorf("folder pattern %q: empty path component", pattern)
	}
	p := &Pattern{raw: pattern}
	seen := map[string]bool{}
	for _, comp := range strings.Split(norm, "/") {
		part, err := parsePart(comp, seen)
		if err != nil {
			return nil, fmt.Errorf("folder pattern %q: %w", pattern, err)
		}
		p.parts = append(p.parts, part)
	}
	need := map[Schedule][]string{ISOWeek: {"year", "week"}, Day: {"year", "month", "day"}, Month: {"year", "month"}}[s]
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
		if n := len(part.tokens); n > 0 && part.tokens[n-1].field != "" {
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

// Depth is the number of path components.
func (p *Pattern) Depth() int { return len(p.parts) }

// Folder returns the folder of period per as a slash-separated relative path.
func (p *Pattern) Folder(per Period) string {
	vals := map[string]int{"year": per.Year, "week": per.Week, "month": per.Month, "day": per.Day}
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
func (p *Pattern) Match(rel string, s Schedule) (Period, bool) {
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
	return FromValues(s, vals["year"], vals["week"], vals["month"], vals["day"])
}

// FromValues builds a period from its numbers and checks that it exists.
func FromValues(s Schedule, year, wk, month, day int) (Period, bool) {
	if year < 1970 || year > 9999 {
		return Period{}, false
	}
	switch s {
	case ISOWeek:
		if wk < 1 || wk > 53 {
			return Period{}, false
		}
		start := isoWeekStart(year, wk, time.Local)
		if y, w := start.ISOWeek(); y != year || w != wk {
			return Period{}, false
		}
		return Period{Schedule: ISOWeek, Year: year, Week: wk, Start: start}, true
	case Day:
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		if month < 1 || month > 12 || t.Day() != day || int(t.Month()) != month {
			return Period{}, false
		}
		return For(t, Day), true
	case Month:
		if month < 1 || month > 12 {
			return Period{}, false
		}
		return For(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local), Month), true
	}
	return Period{}, false
}

var labelRes = map[Schedule]*regexp.Regexp{
	ISOWeek: regexp.MustCompile(`^(\d{4})-?W?(\d{1,2})$`),
	Day:     regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`),
	Month:   regexp.MustCompile(`^(\d{4})-(\d{2})$`),
}

// ParseLabel parses a --week style label: "2026-47" or "2026-W47" for
// weeks, "2026-11-22" for days and "2026-11" for months.
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
	var per Period
	switch s {
	case ISOWeek:
		per, ok = FromValues(s, n[1], n[2], 0, 0)
	case Day:
		per, ok = FromValues(s, n[1], 0, n[2], n[3])
	case Month:
		per, ok = FromValues(s, n[1], 0, n[2], 0)
	}
	if !ok {
		return Period{}, fmt.Errorf("period %q does not exist", label)
	}
	return per, nil
}
