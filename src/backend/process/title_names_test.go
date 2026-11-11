package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestOrganizationGameNameOverrides(t *testing.T) {
	const id, key = "0100000000010000", "0100000000010"
	for _, missingBase := range []bool{false, true} {
		t.Run(map[bool]string{true: "missing base", false: "base"}[missingBase], func(t *testing.T) {
			root := t.TempDir()
			file := db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: "Original.nsp"}, Metadata: previewTestMetadata(id, 0, "1.0")}
			if err := os.WriteFile(filepath.Join(root, file.ExtendedInfo.FileName), []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			game := &db.SwitchGameFiles{BaseExist: !missingBase, File: file, Updates: map[int]db.SwitchFileInfo{}}
			if missingBase {
				file.Metadata = previewTestMetadata("0100000000010800", 2, "2.0")
				game.Updates[2] = file
			}
			local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: game}}
			remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{key: {Attributes: db.TitleAttributes{Id: id, Name: "Catalog"}}}}
			names := map[string]string{id: "Custom 日本語"}
			options := settings.OrganizeOptions{RenameFiles: true, CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}", FileNameTemplate: "{TITLE_NAME} [{TITLE_ID}]", AllowUnicodeFileNames: true, ProcessWhenMissingBaseGame: true}
			plan, err := BuildOrganizationPlan(root, local, remote, options, names)
			if err != nil || len(plan.Moves) != 1 || len(plan.Conflicts) != 0 {
				t.Fatalf("plan = %#v, %v", plan, err)
			}
			if !strings.Contains(plan.Moves[0].Destination, "Custom 日本語") {
				t.Fatal(plan.Moves)
			}
			preview, err := BuildOrganizationPreview(root, options, local, remote, names)
			if err != nil {
				t.Fatal(err)
			}
			kind := "game"
			if missingBase {
				kind = "update"
			}
			for _, row := range preview {
				if row.Kind == kind && filepath.Join(root, row.Path) != plan.Moves[0].Destination {
					t.Fatalf("preview = %v, plan = %v", row, plan.Moves)
				}
			}
			if remote.TitlesMap[key].Attributes.Name != "Catalog" || game.File.ExtendedInfo.FileName != "Original.nsp" {
				t.Fatal("planning changed source data")
			}
			if err := os.MkdirAll(filepath.Dir(plan.Moves[0].Destination), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.Moves[0].Destination, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			blocked, err := BuildOrganizationPlan(root, local, remote, options, names)
			if err != nil || len(blocked.Conflicts) == 0 {
				t.Fatalf("collision = %#v, %v", blocked, err)
			}
		})
	}
}
