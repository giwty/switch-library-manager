package reportexport

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func openReport(t *testing.T, path string) *excelize.File {
	t.Helper()
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	})
	return book
}

func TestXLSXPreservesLiteralValuesAndFormatting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.xlsx")
	longPath := "C:/synthetic/" + strings.Repeat("long folder/", 30) + "test.nsp"
	report := Report{Kind: "games", Headers: []string{"Title", "Title ID", "Update", "Version", "Date", "File", "Enabled", "Blank"}, Rows: [][]any{
		{"日本語 café åäö 🎮", "0100000000001000", 131072, "1.02", "2026-11-02", longPath, true, nil},
		{"=HYPERLINK(\"https://example.invalid\")", "0012345678901234", float64(65536), "+1", "@SUM(A1)", "comma, quote \" and\nnewline", false, "-1"},
	}}
	if err := WriteXLSX(path, report); err != nil {
		t.Fatal(err)
	}
	book := openReport(t, path)
	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"Library"}) {
		t.Fatalf("sheets = %v", got)
	}
	for cell, want := range map[string]string{"A2": "日本語 café åäö 🎮", "B2": "0100000000001000", "B3": "0012345678901234", "C2": "131072", "D2": "1.02", "E2": "2026-11-02", "F2": longPath, "A3": report.Rows[1][0].(string), "F3": "comma, quote \" and\nnewline", "D3": "+1", "E3": "@SUM(A1)", "H3": "-1"} {
		got, err := book.GetCellValue("Library", cell)
		if err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", cell, got, err, want)
		}
		formula, err := book.GetCellFormula("Library", cell)
		if err != nil || formula != "" {
			t.Errorf("%s formula = %q, %v", cell, formula, err)
		}
	}
	for _, cell := range []string{"B2", "B3", "D2", "E2", "A3"} {
		kind, err := book.GetCellType("Library", cell)
		if err != nil || kind != excelize.CellTypeSharedString {
			t.Errorf("%s type = %v, %v", cell, kind, err)
		}
	}
	for _, cell := range []string{"C2", "C3"} {
		kind, err := book.GetCellType("Library", cell)
		// OOXML omits the type attribute for ordinary numeric cells.
		if err != nil || (kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset) {
			t.Errorf("%s numeric type = %v, %v", cell, kind, err)
		}
	}
	tables, err := book.GetTables("Library")
	if err != nil || len(tables) != 1 {
		t.Fatalf("tables = %v, %v", tables, err)
	}
	if tables[0].Range != "A1:H3" || tables[0].StyleName != "TableStyleMedium2" || tables[0].ShowRowStripes == nil || !*tables[0].ShowRowStripes {
		t.Errorf("table = %+v", tables[0])
	}
	panes, err := book.GetPanes("Library")
	if err != nil || !panes.Freeze || panes.YSplit != 1 || panes.TopLeftCell != "A2" {
		t.Errorf("panes = %+v, %v", panes, err)
	}
	styleID, err := book.GetCellStyle("Library", "A1")
	if err != nil {
		t.Fatal(err)
	}
	style, err := book.GetStyle(styleID)
	if err != nil || style.Font == nil || !style.Font.Bold {
		t.Errorf("header style = %+v, %v", style, err)
	}
	width, err := book.GetColWidth("Library", "F")
	if err != nil || width != 60 {
		t.Errorf("path column width = %v, %v", width, err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var tableXML string
	for _, file := range z.File {
		if file.Name == "xl/tables/table1.xml" {
			r, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(r)
			closeErr := r.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("read table: %v, %v", readErr, closeErr)
			}
			tableXML = string(data)
		}
	}
	if !strings.Contains(tableXML, `<autoFilter ref="A1:H3"`) {
		t.Fatalf("missing table filter: %s", tableXML)
	}
}

func TestXLSXAllReportKindsAndFilenames(t *testing.T) {
	date := time.Date(2026, 11, 2, 23, 59, 0, 0, time.FixedZone("local", 3600))
	for kind, sheet := range sheetNames {
		t.Run(kind, func(t *testing.T) {
			filename := Filename(kind, date)
			if filename != "slm_"+kind+".2026-11-02.xlsx" {
				t.Fatal(filename)
			}
			path := filepath.Join(t.TempDir(), filename)
			if err := WriteXLSX(path, Report{Kind: kind, Headers: []string{"Title"}, Rows: [][]any{{"test"}}}); err != nil {
				t.Fatal(err)
			}
			if got := openReport(t, path).GetSheetList(); !reflect.DeepEqual(got, []string{sheet}) {
				t.Fatalf("sheets = %v", got)
			}
		})
	}
}

func TestXLSXRejectsInvalidInputBeforeReplacingDestination(t *testing.T) {
	valid := Report{Kind: "games", Headers: []string{"Title"}, Rows: [][]any{{"test"}}}
	cases := map[string]Report{
		"unknown kind":     {Kind: "../bad", Headers: valid.Headers},
		"no headers":       {Kind: "games"},
		"empty header":     {Kind: "games", Headers: []string{" "}},
		"duplicate header": {Kind: "games", Headers: []string{"Title", "title"}},
		"long header":      {Kind: "games", Headers: []string{strings.Repeat("a", 256)}},
		"too many columns": {Kind: "games", Headers: make([]string, excelize.MaxColumns+1)},
		"too many rows":    {Kind: "games", Headers: valid.Headers, Rows: make([][]any, excelize.TotalRows)},
		"row width":        {Kind: "games", Headers: valid.Headers, Rows: [][]any{{"a", "b"}}},
	}
	for name, value := range map[string]any{"object": map[string]string{"a": "b"}, "array": []string{"x"}, "infinity": math.Inf(1), "nan": math.NaN(), "imprecise integer": 1000000000000000, "long text": strings.Repeat("a", 32768), "long unicode": strings.Repeat("🎮", 16384), "invalid UTF-8": string([]byte{0xff})} {
		cases[name] = Report{Kind: "games", Headers: valid.Headers, Rows: [][]any{{value}}}
	}
	path := filepath.Join(t.TempDir(), "keep.xlsx")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			if err := WriteXLSX(path, report); err == nil {
				t.Fatal("expected validation error")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "original" {
				t.Fatalf("original changed: %q, %v", data, err)
			}
		})
	}
	if err := WriteXLSX(path, valid); err != nil {
		t.Fatal(err)
	}
	openReport(t, path)
}

func TestXLSXFileFailuresCleanUpTemporaryFiles(t *testing.T) {
	report := Report{Kind: "issues", Headers: []string{"Issue"}, Rows: [][]any{{"test"}}}
	dir := t.TempDir()
	for _, path := range []string{"", filepath.Join(dir, "missing", "report.xlsx"), dir} {
		if err := WriteXLSX(path, report); err == nil {
			t.Fatalf("expected failure for %q", path)
		}
	}
	files, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".slm-export-") {
			t.Errorf("temporary file leaked: %s", file.Name())
		}
	}
	file, err := os.CreateTemp(dir, "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkbook(file, report); err == nil {
		t.Fatal("expected write error for closed file")
	}
}

func TestXLSXExceedsLegacyRowLimit(t *testing.T) {
	report := Report{Kind: "missing_games", Headers: []string{"Title ID"}, Rows: make([][]any, 65537)}
	for i := range report.Rows {
		report.Rows[i] = []any{fmt.Sprintf("%016x", i)}
	}
	path := filepath.Join(t.TempDir(), "large.xlsx")
	if err := WriteXLSX(path, report); err != nil {
		t.Fatal(err)
	}
	book := openReport(t, path)
	value, err := book.GetCellValue("Missing games", "A65538")
	if err != nil || value != "0000000000010000" {
		t.Fatalf("last row = %q, %v", value, err)
	}
	tables, err := book.GetTables("Missing games")
	if err != nil || len(tables) != 1 || tables[0].Range != "A1:A65538" {
		t.Fatalf("tables = %v, %v", tables, err)
	}
}
