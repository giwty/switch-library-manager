package consoleapp

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/export"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestConsoleReportsSupportCSVAndXLSXIndependently(t *testing.T) {
	date := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	local := &db.LocalSwitchFilesDB{
		Skipped:   map[db.ExtendedFileInfo]db.SkippedFile{{BaseFolder: "synthetic", FileName: "日本語.nsp"}: {ReasonCode: 7, ReasonText: "test issue"}},
		TitlesMap: map[string]*db.SwitchGameFiles{"0100000000001000": {BaseExist: true}},
	}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{"0100000000001000": {
		Attributes: db.TitleAttributes{Id: "0100000000001000", Name: "日本語 café åäö"},
		Updates:    map[int]string{131072: "2026-11-02"},
		Dlc:        map[string]db.TitleAttributes{"0100000000001001": {Id: "0100000000001001", Name: "Expansion"}},
	}}}
	for _, mode := range []string{"disabled", "csv", "xlsx", "both"} {
		t.Run(mode, func(t *testing.T) {
			folder := t.TempDir()
			c := CreateConsole("", nil, nil)
			c.exportDate = date
			csvEnabled := mode == "csv" || mode == "both"
			xlsxEnabled := mode == "xlsx" || mode == "both"
			if xlsxEnabled {
				c.xlsxOutput = c.prepareExportFolder(filepath.Join(folder, "xlsx", "nested"), "Excel")
			}
			csvPath := func(kind string) string {
				if !csvEnabled {
					return ""
				}
				return filepath.Join(folder, csvExportFilename(kind, date))
			}
			c.processIssues(local, csvPath(csvIssuesKind), nil)
			c.processMissingUpdates(local, remote, &settings.AppSettings{}, csvPath(csvMissingUpdatesKind))
			c.processMissingDLC(local, remote, &settings.AppSettings{}, csvPath(csvMissingDLCKind))
			for kind, sheet := range map[string]string{csvIssuesKind: "Issues", csvMissingUpdatesKind: "Missing updates", csvMissingDLCKind: "Missing DLC"} {
				var csvRows [][]string
				if csvEnabled {
					data, err := os.ReadFile(csvPath(kind))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
						t.Fatal("CSV BOM missing")
					}
					csvRows, err = csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
					if err != nil {
						t.Fatal(err)
					}
					if len(csvRows) != 2 {
						t.Fatalf("CSV rows = %v", csvRows)
					}
				}
				if xlsxEnabled {
					book, err := excelize.OpenFile(filepath.Join(c.xlsxOutput, reportexport.Filename(kind, date)))
					if err != nil {
						t.Fatal(err)
					}
					rows, err := book.GetRows(sheet)
					if err != nil {
						t.Fatal(err)
					}
					if csvEnabled && !reflect.DeepEqual(rows, csvRows) {
						t.Errorf("Excel rows %v differ from CSV %v", rows, csvRows)
					}
					if len(rows) != 2 {
						t.Fatalf("Excel rows = %v", rows)
					}
					if kind == csvMissingUpdatesKind {
						cellType, err := book.GetCellType(sheet, "D2")
						if err != nil || (cellType != excelize.CellTypeNumber && cellType != excelize.CellTypeUnset) {
							t.Errorf("update type = %v, %v", cellType, err)
						}
						if rows[1][1] != "0100000000001000" || rows[1][3] != "131072" {
							t.Errorf("update row = %v", rows[1])
						}
					}
					if err := book.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "disabled" {
				files, err := os.ReadDir(folder)
				if err != nil || len(files) != 0 {
					t.Fatalf("disabled export created files: %v, %v", files, err)
				}
			}
		})
	}
}

func TestConsoleExportFailuresDoNotSuppressOtherFormat(t *testing.T) {
	folder := t.TempDir()
	core, logs := observer.New(zap.ErrorLevel)
	c := CreateConsole("", zap.New(core).Sugar(), nil)
	c.exportDate = time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)
	report := reportexport.Report{Kind: "issues", Headers: []string{"Issue"}, Rows: [][]any{{"test"}}}
	c.xlsxOutput = filepath.Join(folder, "missing")
	csvPath := filepath.Join(folder, "issues.csv")
	c.exportReport(csvPath, report)
	if _, err := os.Stat(csvPath); err != nil {
		t.Fatal("CSV suppressed", err)
	}
	if logs.Len() != 1 {
		t.Fatalf("Excel failure was not logged: %v", logs.All())
	}
	c.xlsxOutput = folder
	c.exportReport(filepath.Join(folder, "missing", "issues.csv"), report)
	if _, err := os.Stat(filepath.Join(folder, reportexport.Filename("issues", c.exportDate))); err != nil {
		t.Fatal("Excel suppressed", err)
	}
	if got := c.prepareExportFolder(csvPath, "Excel"); got != "" {
		t.Errorf("file accepted as folder: %q", got)
	}
}

func TestConsoleSkipsEmptyReports(t *testing.T) {
	folder := t.TempDir()
	c := CreateConsole("", nil, nil)
	c.xlsxOutput = folder
	local := &db.LocalSwitchFilesDB{}
	remote := &db.SwitchTitlesDB{}
	c.processIssues(local, filepath.Join(folder, "issues.csv"), nil)
	c.processMissingUpdates(local, remote, &settings.AppSettings{}, filepath.Join(folder, "updates.csv"))
	c.processMissingDLC(local, remote, &settings.AppSettings{}, filepath.Join(folder, "dlc.csv"))
	files, err := os.ReadDir(folder)
	if err != nil || len(files) != 0 {
		t.Fatalf("empty report exports = %v, %v", files, err)
	}
}
