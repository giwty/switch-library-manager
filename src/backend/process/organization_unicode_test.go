package process

import (
	"testing"

	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestUnicodeFileAndFolderNames(t *testing.T) {
	for _, test := range []struct {
		name   string
		legacy string
	}{
		{"日本語", ""},
		{"한국어", ""},
		{"中文", ""},
		{"åäö", ""},
		{"🎮", ""},
		{"Pokémon™", "Pokémon"},
		{"マリオ", "mario"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := map[string]string{settings.TEMPLATE_TITLE_NAME: test.name}
			options := settings.OrganizeOptions{
				RenameFiles: true, FileNameTemplate: "{TITLE_NAME}", FolderNameTemplate: "{TITLE_NAME}",
				AllowUnicodeFileNames: true,
			}
			if got := getFolderName(options, data); got != test.name {
				t.Fatalf("Unicode folder = %q, want %q", got, test.name)
			}
			if got := getFileName(options, "original.nsp", data, 0); got != test.name+".nsp" {
				t.Fatalf("Unicode filename = %q", got)
			}
			options.AllowUnicodeFileNames = false
			if got := getFolderName(options, data); got != test.legacy {
				t.Fatalf("legacy folder = %q, want %q", got, test.legacy)
			}
			want := test.legacy
			if want != "" {
				want += ".nsp"
			}
			if got := getFileName(options, "original.nsp", data, 0); got != want {
				t.Fatalf("legacy filename = %q, want %q", got, want)
			}
		})
	}
}
