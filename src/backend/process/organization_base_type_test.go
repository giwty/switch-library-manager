package process

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestIncludeBaseTypeMatchesPreviewPlanAndExecution(t *testing.T) {
	for _, include := range []bool{false, true} {
		for _, safe := range []bool{false, true} {
			t.Run(fmt.Sprintf("include=%v/safe=%v", include, safe), func(t *testing.T) {
				root := t.TempDir()
				const baseID, updateID, dlcID = "0100E95004039000", "0100E95004039800", "0100E9500403A001"
				file := func(name, id string, version int) db.SwitchFileInfo {
					writeFixture(t, filepath.Join(root, name), name)
					return db.SwitchFileInfo{
						ExtendedInfo: db.ExtendedFileInfo{BaseFolder: root, FileName: name},
						Metadata:     contentMetadata(id, version, "1.0.0"),
					}
				}
				base := file("base.xci", baseID, 0)
				bundledUpdate := base
				bundledUpdate.Metadata = contentMetadata(updateID, 1, "1.1.0")
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
					"0100e95004039": {
						BaseExist: true,
						File:      base,
						Updates: map[int]db.SwitchFileInfo{
							1: bundledUpdate,
							2: file("update.nsp", updateID, 2),
						},
						Dlc: map[string]db.SwitchFileInfo{dlcID: file("dlc.nsp", dlcID, 0)},
					},
				}}
				options := settings.OrganizeOptions{
					RenameFiles:           true,
					IncludeBaseType:       include,
					AllowUnicodeFileNames: !safe,
					FileNameTemplate:      "{TITLE_ID}[{TYPE}]({TYPE})<{TYPE}> ({PACKAGE_CONTENTS})",
				}
				baseType := ""
				if include {
					baseType = "[BASE](BASE)BASE"
				}
				want := map[string]string{
					"game":   baseID + baseType + " (1G+1U).xci",
					"update": updateID + "[UPD](UPD)UPD.nsp",
					"dlc":    dlcID + "[DLC](DLC)DLC.nsp",
				}
				preview, err := BuildOrganizationPreview(root, options, local, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(preview) != len(want) {
					t.Fatalf("preview = %#v, want three physical files", preview)
				}
				for _, entry := range preview {
					if entry.Path != want[entry.Kind] {
						t.Errorf("%s preview = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
					}
				}
				plan, err := BuildOrganizationPlan(root, local, nil, options)
				if err != nil {
					t.Fatal(err)
				}
				if len(plan.Conflicts) != 0 || len(plan.Moves) != len(want) {
					t.Fatalf("unexpected plan: %#v", plan)
				}
				if err := ExecuteOrganizationPlan(plan, local, root, options, nil); err != nil {
					t.Fatal(err)
				}
				assertMoved(t, filepath.Join(root, "base.xci"), filepath.Join(root, want["game"]), "base.xci")
				assertMoved(t, filepath.Join(root, "update.nsp"), filepath.Join(root, want["update"]), "update.nsp")
				assertMoved(t, filepath.Join(root, "dlc.nsp"), filepath.Join(root, want["dlc"]), "dlc.nsp")
				plan, err = BuildOrganizationPlan(root, local, nil, options)
				if err != nil || len(plan.Conflicts) != 0 || len(plan.Moves) != 0 {
					t.Fatalf("repeat plan = %#v, error = %v; want no changes", plan, err)
				}
			})
		}
	}
}

func TestIncludeBaseTypePreservesLiteralTextAndOriginalNames(t *testing.T) {
	data := map[string]string{settings.TEMPLATE_TITLE_NAME: "BASE Game", settings.TEMPLATE_TYPE: "BASE"}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}[{TYPE}] BASE"}
	if got := getFileName(options, "original.xci", data, 0); got != "BASE Game BASE.xci" {
		t.Fatalf("filename = %q, want literal BASE text preserved", got)
	}
	if data[settings.TEMPLATE_TYPE] != "BASE" {
		t.Fatal("filename generation changed the content type metadata")
	}
	options.RenameFiles = false
	if got := getFileName(options, "original[BASE].xci", data, 0); got != "original[BASE].xci" {
		t.Fatalf("filename = %q, want original filename when renaming is disabled", got)
	}
}
