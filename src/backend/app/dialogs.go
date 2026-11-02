package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SelectFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Select games folder"})
}

func (a *App) ConfirmOrganization() (bool, error) {
	return confirmOrganization(a.ctx, wailsruntime.MessageDialog)
}

func confirmOrganization(ctx context.Context, showDialog func(context.Context, wailsruntime.MessageDialogOptions) (string, error)) (bool, error) {
	selection, err := showDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Confirmation",
		Message:       "Are you sure you want to begin library organization?\n\nThis action will modify your local library files",
		Buttons:       []string{"Yes", "No"},
		DefaultButton: "Yes",
		CancelButton:  "No",
	})
	if err != nil {
		return false, err
	}
	return selection == "Yes", nil
}

func (a *App) ShowMessage(kind, title, message, detail string) error {
	dialogType := wailsruntime.InfoDialog
	switch kind {
	case "error":
		dialogType = wailsruntime.ErrorDialog
	case "warning":
		dialogType = wailsruntime.WarningDialog
	case "question":
		dialogType = wailsruntime.QuestionDialog
	}
	if detail != "" {
		message += "\n\n" + detail
	}
	_, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
	return err
}

func (a *App) ShowInFolder(path string) error {
	if path == "" {
		return errors.New("path is empty")
	}
	command, args := fileManagerCommand(path)
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("show %q in file manager: %w", path, err)
	}
	return nil
}

func fileManagerCommand(path string) (string, []string) {
	cleanPath := filepath.Clean(path)
	switch runtime.GOOS {
	case "windows":
		return "explorer.exe", []string{"/select,", cleanPath}
	case "darwin":
		return "open", []string{"-R", cleanPath}
	default:
		return "xdg-open", []string{filepath.Dir(cleanPath)}
	}
}
