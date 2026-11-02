package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trembon/switch-library-manager/backend/export"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestSaveXLSXDialogAndErrors(t *testing.T) {
	report := reportexport.Report{Kind: "missing_updates", Headers: []string{"Title"}, Rows: [][]any{{"日本語"}}}
	date := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	failure := errors.New("synthetic failure")
	for _, test := range []struct {
		name, path          string
		dialogErr, writeErr error
		wantWrite           bool
		wantErr             bool
	}{
		{name: "cancel"},
		{name: "dialog failure", dialogErr: failure, wantErr: true},
		{name: "save", path: filepath.Join(t.TempDir(), "report.xlsx"), wantWrite: true},
		{name: "append extension", path: filepath.Join(t.TempDir(), "report"), wantWrite: true},
		{name: "uppercase extension", path: filepath.Join(t.TempDir(), "report.XLSX"), wantWrite: true},
		{name: "wrong extension", path: "report.xls", wantErr: true},
		{name: "write failure", path: "report.xlsx", writeErr: failure, wantWrite: true, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			written := false
			path, err := saveXLSX(context.Background(), report, date, func(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
				if opts.DefaultFilename != "slm_missing_updates.2026-11-02.xlsx" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.xlsx" {
					t.Fatalf("dialog = %+v", opts)
				}
				return test.path, test.dialogErr
			}, func(path string, got reportexport.Report) error {
				written = true
				if filepath.Ext(path) != ".xlsx" && filepath.Ext(path) != ".XLSX" || got.Rows[0][0] != "日本語" {
					t.Fatalf("write = %s, %+v", path, got)
				}
				return test.writeErr
			})
			if (err != nil) != test.wantErr || written != test.wantWrite {
				t.Fatalf("path=%q err=%v written=%v", path, err, written)
			}
			if (test.wantErr || !test.wantWrite) && path != "" {
				t.Fatalf("failed/canceled export returned path %q", path)
			}
			if test.wantWrite && !test.wantErr && path == "" {
				t.Fatal("saved path missing")
			}
		})
	}
}

func TestSaveXLSXRejectsInvalidReportBeforeDialog(t *testing.T) {
	_, err := saveXLSX(context.Background(), reportexport.Report{}, time.Now(), func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		t.Fatal("unexpected dialog")
		return "", nil
	}, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSaveXLSXDoesNotBypassOverwritePromptWhenAppendingExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing")
	if err := os.WriteFile(path+".xlsx", []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	report := reportexport.Report{Kind: "issues", Headers: []string{"Issue"}}
	_, err := saveXLSX(context.Background(), report, time.Now(), func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return path, nil
	}, func(string, reportexport.Report) error {
		t.Fatal("existing suffixed file must not be overwritten without a native prompt")
		return nil
	})
	if err == nil {
		t.Fatal("expected error asking for the full filename")
	}
}

func TestExportXLSXRejectsConcurrentExport(t *testing.T) {
	app := &App{}
	app.exportMu.Lock()
	defer app.exportMu.Unlock()
	if _, err := app.ExportXLSX(reportexport.Report{}); err == nil {
		t.Fatal("concurrent export was accepted")
	}
}
