package app

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestConfirmOrganization(t *testing.T) {
	dialogErr := errors.New("dialog unavailable")
	for _, test := range []struct {
		name      string
		selection string
		dialogErr error
		want      bool
	}{
		{name: "yes confirms", selection: "Yes", want: true},
		{name: "no cancels", selection: "No"},
		{name: "empty response cancels"},
		{name: "ok does not confirm", selection: "Ok"},
		{name: "lowercase yes does not confirm", selection: "yes"},
		{name: "unexpected response cancels", selection: "Continue"},
		{name: "dialog error cancels", dialogErr: dialogErr},
		{name: "yes with dialog error cancels", selection: "Yes", dialogErr: dialogErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			confirmed, err := confirmOrganization(ctx, func(gotCtx context.Context, options wailsruntime.MessageDialogOptions) (string, error) {
				calls++
				if gotCtx != ctx {
					t.Fatal("confirmation did not forward the application context")
				}
				if options.Type != wailsruntime.QuestionDialog || options.Title != "Confirmation" ||
					options.Message != "Are you sure you want to begin library organization?\n\nThis action will modify your local library files" {
					t.Fatalf("unexpected confirmation dialog: %#v", options)
				}
				if !reflect.DeepEqual(options.Buttons, []string{"Yes", "No"}) || options.DefaultButton != "Yes" || options.CancelButton != "No" {
					t.Fatalf("confirmation must explicitly offer Yes/No with the expected keyboard defaults: %#v", options)
				}
				return test.selection, test.dialogErr
			})
			if calls != 1 {
				t.Fatalf("dialog called %d times; want once", calls)
			}
			if confirmed != test.want || err != test.dialogErr {
				t.Fatalf("confirmOrganization() = %t, %v; want %t, %v", confirmed, err, test.want, test.dialogErr)
			}
		})
	}
}

func TestFileManagerCommand(t *testing.T) {
	path := filepath.Join("library with spaces", "Pokémon [0100000000001000].nsp")
	command, args := fileManagerCommand(path)
	cleanPath := filepath.Clean(path)

	var wantCommand string
	var wantArgs []string
	switch runtime.GOOS {
	case "windows":
		wantCommand = "explorer.exe"
		wantArgs = []string{"/select,", cleanPath}
	case "darwin":
		wantCommand = "open"
		wantArgs = []string{"-R", cleanPath}
	default:
		wantCommand = "xdg-open"
		wantArgs = []string{filepath.Dir(cleanPath)}
	}
	if command != wantCommand || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("fileManagerCommand(%q) = %q, %#v; want %q, %#v", path, command, args, wantCommand, wantArgs)
	}
}

func TestShowInFolderRejectsEmptyPath(t *testing.T) {
	if err := (&App{}).ShowInFolder(""); err == nil || err.Error() != "path is empty" {
		t.Fatalf("ShowInFolder(\"\") error = %v; want path is empty", err)
	}
}
