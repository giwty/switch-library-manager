package consoleapp

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/export"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/xuri/excelize/v2"
)

func TestConsoleDateFormatMatchesCSVAndExcel(t *testing.T) {
	for _, item := range []struct{ pattern, date, want string }{
		{"", "2026-11-23", "2026-11-23"},
		{"MM/DD/YYYY", "2026-11-23", "11/23/2026"},
		{"DD/MM/YYYY", "2026-11-23", "23/11/2026"},
		{"D.M.YYYY", "2026-02-09", "9.2.2026"},
		{"MMMM D, YYYY", "2026-11-23", "November 23, 2026"},
		{"D MMM YYYY", "2024-02-29", "29 Feb 2024"},
		{"DD/MM/YYYY", "-", ""},
		{"DD/MM/YYYY", "2026-02-29", ""},
	} {
		t.Run(item.pattern+item.date, func(t *testing.T) {
			folder := t.TempDir()
			local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{"0100000000001000": {BaseExist: true}}}
			remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100000000001000": {
				Attributes: db.TitleAttributes{Id: "0100000000001000", Name: "Synthetic date test"}, Updates: map[int]string{65536: item.date},
			}}}
			c := CreateConsole("", nil, nil)
			c.exportDate = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
			c.xlsxOutput = folder
			csvPath := filepath.Join(folder, csvExportFilename(csvMissingUpdatesKind, c.exportDate))
			output, err := os.CreateTemp(folder, "stdout-*")
			if err != nil {
				t.Fatal(err)
			}
			previous := os.Stdout
			func() {
				os.Stdout = output
				defer func() { os.Stdout = previous }()
				c.processMissingUpdates(local, remote, &settings.AppSettings{GUI: settings.GUISettings{DateFormat: item.pattern}}, csvPath)
			}()
			if err := output.Close(); err != nil {
				t.Fatal(err)
			}
			printed, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			wantDisplay := item.want
			if wantDisplay == "" {
				wantDisplay = "Unknown"
			}
			if !strings.Contains(string(printed), wantDisplay) {
				t.Fatalf("console did not show %q: %s", wantDisplay, printed)
			}
			data, err := os.ReadFile(csvPath)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
			if err != nil || len(rows) != 2 || rows[1][4] != item.want {
				t.Fatalf("CSV = %v, %v", rows, err)
			}
			book, err := excelize.OpenFile(filepath.Join(folder, reportexport.Filename(csvMissingUpdatesKind, c.exportDate)))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := book.Close(); err != nil {
					t.Error(err)
				}
			}()
			value, err := book.GetCellValue("Missing updates", "E2")
			if err != nil || value != item.want {
				t.Fatalf("Excel = %q, %v", value, err)
			}
			if item.want != "" {
				cellType, err := book.GetCellType("Missing updates", "E2")
				if err != nil || cellType != excelize.CellTypeSharedString {
					t.Fatalf("date must be literal text: %v, %v", cellType, err)
				}
			}
			if remote.TitlesMap["0100000000001000"].Updates[65536] != item.date {
				t.Fatal("source date was changed")
			}
		})
	}
}
