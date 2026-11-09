// Package dateformat formats calendar dates for presentation without changing source data.
package dateformat

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

const Default = "YYYY-MM-DD"

var (
	patternRE = regexp.MustCompile(`^(YYYY|MMMM|MMM|MM|M|DD|D)[ /.,-]+(YYYY|MMMM|MMM|MM|M|DD|D)[ /.,-]+(YYYY|MMMM|MMM|MM|M|DD|D)$`)
	tokenRE   = regexp.MustCompile(`YYYY|MMMM|MMM|MM|M|DD|D`)
	isoRE     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// Normalize supplies the default and removes surrounding ASCII whitespace.
func Normalize(pattern string) string {
	pattern = strings.Trim(pattern, " \t\r\n\v\f")
	if pattern == "" {
		return Default
	}
	return pattern
}

// Formatter uses English month names and never converts between time zones.
type Formatter struct {
	layout string
}

// New validates the supported date-only tokens and builds an immutable formatter.
func New(pattern string) (Formatter, error) {
	pattern = Normalize(pattern)
	parts := patternRE.FindStringSubmatch(pattern)
	valid := len(pattern) <= 32 && len(parts) == 4
	seen := map[byte]bool{}
	if valid {
		for _, part := range parts[1:] {
			if seen[part[0]] {
				valid = false
			}
			seen[part[0]] = true
		}
	}
	if !valid {
		return Formatter{}, errors.New("use one YYYY, one M/MM/MMM/MMMM and one D/DD separated by spaces or / - . , (maximum 32 characters)")
	}
	layouts := map[string]string{"YYYY": "2006", "M": "1", "MM": "01", "MMM": "Jan", "MMMM": "January", "D": "2", "DD": "02"}
	return Formatter{layout: tokenRE.ReplaceAllStringFunc(pattern, func(token string) string { return layouts[token] })}, nil
}

// FormatISO returns an empty string for missing or malformed source dates.
// Callers choose the appropriate unavailable-date label for their presentation.
func (f Formatter) FormatISO(value string) string {
	if f.layout == "" || !isoRE.MatchString(value) {
		return ""
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ""
	}
	return date.Format(f.layout)
}
