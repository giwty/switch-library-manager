package dateformat

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestSharedDateFormatContract(t *testing.T) {
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	type example struct{ Pattern, Date, Expected string }
	var cases struct {
		Formats         []example
		Dates           []example
		InvalidPatterns []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases.Formats {
		formatter, err := New(item.Pattern)
		if err != nil {
			t.Fatalf("New(%q): %v", item.Pattern, err)
		}
		if got := formatter.FormatISO("2026-11-23"); got != item.Expected {
			t.Errorf("format %q = %q, want %q", item.Pattern, got, item.Expected)
		}
	}
	for _, item := range cases.Dates {
		formatter, err := New(item.Pattern)
		if err != nil {
			t.Fatal(err)
		}
		if got := formatter.FormatISO(item.Date); got != item.Expected {
			t.Errorf("date %q = %q, want %q", item.Date, got, item.Expected)
		}
	}
	for _, pattern := range cases.InvalidPatterns {
		if _, err := New(pattern); err == nil {
			t.Errorf("accepted invalid pattern %q", pattern)
		}
	}
	if got := (Formatter{}).FormatISO("2026-11-23"); got != "" {
		t.Fatalf("zero formatter = %q", got)
	}
}

func TestCalendarDatesDoNotDependOnLocalTimezone(t *testing.T) {
	previous := time.Local
	t.Cleanup(func() { time.Local = previous })
	formatter, err := New(Default)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-12, 0, 14} {
		time.Local = time.FixedZone("synthetic", offset*3600)
		if got := formatter.FormatISO("2026-03-29"); got != "2026-03-29" {
			t.Errorf("offset %d = %q", offset, got)
		}
	}
}
