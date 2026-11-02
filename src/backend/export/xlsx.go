// Package reportexport writes tabular reports without depending on either UI adapter.
package reportexport

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Report contains ordered headers and literal string, numeric, boolean or blank cells.
// Strings are never interpreted as formulas or converted to numbers or dates.
type Report struct {
	Kind    string   `json:"kind"`
	Headers []string `json:"headers"`
	Rows    [][]any  `json:"rows"`
}

var sheetNames = map[string]string{
	"games": "Library", "missing_games": "Missing games", "missing_updates": "Missing updates",
	"missing_dlc": "Missing DLC", "issues": "Issues",
}

func Filename(kind string, date time.Time) string {
	return fmt.Sprintf("slm_%s.%s.xlsx", kind, date.Format("2006-01-02"))
}

// Validate checks limits before Excelize can silently truncate cell contents.
func (r Report) Validate() error {
	if _, ok := sheetNames[r.Kind]; !ok {
		return fmt.Errorf("unknown export report kind %q", r.Kind)
	}
	if len(r.Headers) == 0 || len(r.Headers) > excelize.MaxColumns {
		return fmt.Errorf("export must contain 1–%d columns", excelize.MaxColumns)
	}
	if len(r.Rows) >= excelize.TotalRows {
		return fmt.Errorf("export exceeds %d data rows (plus the header)", excelize.TotalRows-1)
	}
	headers := make(map[string]bool, len(r.Headers))
	for column, header := range r.Headers {
		if err := validateText(header, 255); err != nil {
			return fmt.Errorf("export header %d: %w", column+1, err)
		}
		key := strings.ToLower(strings.TrimSpace(header))
		if key == "" || headers[key] {
			return fmt.Errorf("export header %d must be nonempty and unique", column+1)
		}
		headers[key] = true
	}
	for rowIndex, row := range r.Rows {
		if len(row) != len(r.Headers) {
			return fmt.Errorf("export row %d has %d cells; expected %d", rowIndex+2, len(row), len(r.Headers))
		}
		for column, value := range row {
			var err error
			switch v := value.(type) {
			case nil, bool:
			case string:
				err = validateText(v, excelize.TotalCellChars)
			case int:
				err = validateNumber(float64(v))
			case float64:
				err = validateNumber(v)
			default:
				err = fmt.Errorf("unsupported cell type %T", value)
			}
			if err != nil {
				return fmt.Errorf("export row %d, column %d: %w", rowIndex+2, column+1, err)
			}
		}
	}
	return nil
}

func validateText(value string, limit int) error {
	if !utf8.ValidString(value) {
		return errors.New("text is not valid UTF-8")
	}
	units := 0
	for _, character := range value {
		units++
		if character > 0xffff {
			units++
		}
		if units > limit {
			return fmt.Errorf("text exceeds Excel's %d UTF-16 character limit", limit)
		}
	}
	return nil
}

func validateNumber(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 999999999999999 {
		return errors.New("number is nonfinite or exceeds Excel's 15-digit precision; export identifiers as text")
	}
	return nil
}

// WriteXLSX replaces the destination only after a complete workbook has been written
// and closed. The temporary file is in the destination directory for same-volume rename.
func WriteXLSX(destination string, report Report) (err error) {
	if err := report.Validate(); err != nil {
		return err
	}
	if destination == "" {
		return errors.New("Excel export destination is empty")
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".slm-export-*.xlsx")
	if err != nil {
		return fmt.Errorf("create temporary Excel export: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(temp.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove temporary Excel export: %w", removeErr))
		}
	}()
	writeErr := writeWorkbook(temp, report)
	closeErr := temp.Close()
	if writeErr != nil || closeErr != nil {
		return fmt.Errorf("write Excel export %q: %w", destination, errors.Join(writeErr, closeErr))
	}
	if err := os.Rename(temp.Name(), destination); err != nil {
		return fmt.Errorf("replace Excel export %q: %w", destination, err)
	}
	return nil
}

func writeWorkbook(output *os.File, report Report) (err error) {
	book := excelize.NewFile()
	defer func() {
		if closeErr := book.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close Excel workbook: %w", closeErr))
		}
	}()
	sheet := sheetNames[report.Kind]
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}
	headerStyle, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	if err != nil {
		return err
	}
	bodyStyle, err := book.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	if err != nil {
		return err
	}
	widths := make([]int, len(report.Headers))
	for column, header := range report.Headers {
		cell, cellErr := excelize.CoordinatesToCellName(column+1, 1)
		if cellErr != nil {
			return cellErr
		}
		if err := book.SetCellStr(sheet, cell, header); err != nil {
			return err
		}
		widths[column] = textWidth(header)
	}
	for rowIndex, row := range report.Rows {
		for column, value := range row {
			cell, cellErr := excelize.CoordinatesToCellName(column+1, rowIndex+2)
			if cellErr != nil {
				return cellErr
			}
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return fmt.Errorf("set cell %s: %w", cell, err)
			}
			if value != nil {
				widths[column] = max(widths[column], textWidth(fmt.Sprint(value)))
			}
		}
	}
	lastColumn, err := excelize.ColumnNumberToName(len(report.Headers))
	if err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return err
	}
	if len(report.Rows) > 0 {
		lastCell := fmt.Sprintf("%s%d", lastColumn, len(report.Rows)+1)
		if err := book.SetCellStyle(sheet, "A2", lastCell, bodyStyle); err != nil {
			return err
		}
		if err := book.AddTable(sheet, &excelize.Table{Range: "A1:" + lastCell, Name: "SLMReport", StyleName: "TableStyleMedium2", ShowRowStripes: boolPointer(true)}); err != nil {
			return err
		}
	}
	if err := book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	for column, width := range widths {
		name, err := excelize.ColumnNumberToName(column + 1)
		if err != nil {
			return err
		}
		if err := book.SetColWidth(sheet, name, name, float64(min(60, max(12, width+3)))); err != nil {
			return err
		}
	}
	return book.Write(output)
}

func textWidth(text string) int {
	width := 0
	for _, line := range strings.Split(text, "\n") {
		width = max(width, utf8.RuneCountInString(line))
	}
	return width
}

func boolPointer(value bool) *bool { return &value }
