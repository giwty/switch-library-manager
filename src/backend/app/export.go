package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trembon/switch-library-manager/backend/export"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportXLSX saves a snapshot supplied by the frontend's table export adapter.
// It does not read or lock mutable scan state while the native dialog is open.
func (a *App) ExportXLSX(request reportexport.Report) (string, error) {
	if !a.exportMu.TryLock() {
		return "", errors.New("an Excel export is already in progress")
	}
	defer a.exportMu.Unlock()
	return saveXLSX(a.ctx, request, time.Now(), wailsruntime.SaveFileDialog, reportexport.WriteXLSX)
}

func saveXLSX(ctx context.Context, request reportexport.Report, date time.Time,
	dialog func(context.Context, wailsruntime.SaveDialogOptions) (string, error),
	write func(string, reportexport.Report) error,
) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	path, err := dialog(ctx, wailsruntime.SaveDialogOptions{
		Title: "Export to Excel", DefaultFilename: reportexport.Filename(request.Kind, date),
		Filters:              []wailsruntime.FileFilter{{DisplayName: "Excel Workbook (*.xlsx)", Pattern: "*.xlsx"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("select Excel export destination: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if filepath.Ext(path) == "" {
		path += ".xlsx"
		// The native dialog only confirmed the extensionless path. Do not
		// overwrite an existing suffixed file without its overwrite prompt.
		if _, err := os.Stat(path); err == nil {
			return "", errors.New("Excel destination already exists; select its full .xlsx filename to confirm replacement")
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("check Excel export destination: %w", err)
		}
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return "", errors.New("Excel export filename must end in .xlsx")
	}
	if err := write(path, request); err != nil {
		return "", err
	}
	return path, nil
}
