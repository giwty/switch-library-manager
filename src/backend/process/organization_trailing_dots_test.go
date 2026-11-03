package process

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestApplyTemplateTrailingDots(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"Game.", "Game"},
		{"Game...", "Game"},
		{"Game. . \t", "Game"},
		{"Game... ?", "Game"},
		{"Game...???", "Game"},
		{"BIT.TRIP", "BIT.TRIP"},
		{"Game... [{TITLE_ID}]", "Game... [0100E95004039000]"},
		{".Game...", ".Game"},
		{".", ""},
		{"..", ""},
		{"... . \t?", ""},
	} {
		for _, safe := range []bool{false, true} {
			t.Run(test.input+"/"+strconv.FormatBool(safe), func(t *testing.T) {
				data := map[string]string{settings.TEMPLATE_TITLE_ID: "0100e95004039000"}
				if got := applyTemplate(data, safe, test.input, 0); got != test.want {
					t.Fatalf("applyTemplate() = %q, want %q", got, test.want)
				}
				wantSuffix := test.want
				if wantSuffix != "" {
					wantSuffix += "(2)"
				}
				if got := applyTemplate(data, safe, test.input, 2); got != wantSuffix {
					t.Fatalf("applyTemplate() with suffix = %q, want %q", got, wantSuffix)
				}
			})
		}
	}
}

func TestTrailingDotsAfterSafeNameFiltering(t *testing.T) {
	for _, test := range []struct {
		input string
		safe  string
		plain string
	}{
		{"Game...~~", "Game", "Game...~~"},
		{"Game...™", "Game", "Game...™"},
		{"Know by heart…", "Know by heart", "Know by heart…"},
		{"Oh…Sir! The Insult Simulator", "OhSir The Insult Simulator", "Oh…Sir! The Insult Simulator"},
		{"™...", "", "™"},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := applyTemplate(nil, true, test.input, 0); got != test.safe {
				t.Fatalf("safe name = %q, want %q", got, test.safe)
			}
			if got := applyTemplate(nil, false, test.input, 0); got != test.plain {
				t.Fatalf("unfiltered name = %q, want %q", got, test.plain)
			}
		})
	}
}

func TestTrailingDotFileStemsKeepExtensions(t *testing.T) {
	for _, safe := range []bool{false, true} {
		for _, original := range []string{"source.nsp", "source.NSZ", "source.xci", "source.xcz"} {
			t.Run(strconv.FormatBool(safe)+"/"+original, func(t *testing.T) {
				data := map[string]string{settings.TEMPLATE_TITLE_NAME: "Game..."}
				options := settings.OrganizeOptions{RenameFiles: true, AllowUnicodeFileNames: !safe, FileNameTemplate: "{TITLE_NAME}"}
				want := "Game(2)" + filepath.Ext(original)
				if got := getFileName(options, original, data, 2); got != want {
					t.Fatalf("file name = %q, want %q", got, want)
				}
				data[settings.TEMPLATE_TITLE_NAME] = "..."
				if got := getFileName(options, original, data, 2); got != "" {
					t.Fatalf("empty stem became %q", got)
				}
				options.RenameFiles = false
				untouched := "Game..." + filepath.Ext(original)
				if got := getFileName(options, untouched, data, 2); got != untouched {
					t.Fatalf("rename disabled: name = %q, want %q", got, untouched)
				}
			})
		}
	}
}

func TestOrganizationTrailingDots(t *testing.T) {
	const prince = "The Prince of Tennis Sweet School Festival ~~♡-40 and more...~~"
	for _, test := range []struct {
		name  string
		title string
		plain string
		safe  string
	}{
		{"dots", "Adventure...", "Adventure", "Adventure"},
		{"prince", prince, prince, "The Prince of Tennis Sweet School Festival -40 and more"},
	} {
		for _, safe := range []bool{false, true} {
			t.Run(test.name+"/"+strconv.FormatBool(safe), func(t *testing.T) {
				root := t.TempDir()
				incoming := filepath.Join(root, "in")
				mustMkdir(t, incoming)
				file := func(name, id string, version int) db.SwitchFileInfo {
					path := filepath.Join(incoming, name)
					writeFixture(t, path, "synthetic "+name)
					return db.SwitchFileInfo{ExtendedInfo: extendedInfoForPath(t, path), Metadata: metadataForID(id, version)}
				}
				const baseID, updateID, dlcID, key = "0100E95004039000", "0100E95004039800", "0100E9500403A001", "0100e95004039"
				game := &db.SwitchGameFiles{
					BaseExist: true,
					File:      file("base.nsp", baseID, 0),
					Updates:   map[int]db.SwitchFileInfo{5: file("update.NSZ", updateID, 5)},
					Dlc:       map[string]db.SwitchFileInfo{dlcID: file("dlc.xci", dlcID, 0)},
				}
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: game}}
				remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
					key: {Attributes: db.TitleAttributes{Id: baseID, Name: test.title}},
				}}
				options := settings.OrganizeOptions{
					RenameFiles: true, FileNameTemplate: "{TITLE_ID}-{TYPE}...", IncludeBaseType: true,
					CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}", AllowUnicodeFileNames: !safe,
					UpdatesFolder: "updates", DlcFolder: "dlc", DeleteEmptyFolders: true,
				}
				folder := test.plain
				if safe {
					folder = test.safe
				}
				want := map[string]string{
					"game":   filepath.Join(folder, baseID+"-BASE.nsp"),
					"update": filepath.Join(folder, "updates", updateID+"-UPD.NSZ"),
					"dlc":    filepath.Join(folder, "dlc", dlcID+"-DLC.xci"),
				}
				preview, err := BuildOrganizationPreview(root, options, local, remote)
				if err != nil {
					t.Fatal(err)
				}
				if len(preview) != len(want) {
					t.Fatalf("preview = %#v, want all three content kinds", preview)
				}
				for _, entry := range preview {
					if entry.Path != want[entry.Kind] {
						t.Fatalf("%s preview = %q, want %q", entry.Kind, entry.Path, want[entry.Kind])
					}
				}
				plan, err := BuildOrganizationPlan(root, local, remote, options)
				if err != nil || len(plan.Conflicts) != 0 || len(plan.Moves) != 3 {
					t.Fatalf("plan = %#v, err = %v", plan, err)
				}
				sources := map[string]string{"game": "base.nsp", "update": "update.NSZ", "dlc": "dlc.xci"}
				for _, move := range plan.Moves {
					matched := false
					for kind, source := range sources {
						if move.Source == filepath.Join(incoming, source) && move.Destination == filepath.Join(root, want[kind]) {
							matched = true
						}
					}
					if !matched {
						t.Fatalf("move does not match preview: %#v", move)
					}
				}
				if err := ExecuteOrganizationPlan(plan, local, root, options, nil); err != nil {
					t.Fatal(err)
				}
				for kind, source := range sources {
					assertNotExists(t, filepath.Join(incoming, source))
					if got, err := os.ReadFile(filepath.Join(root, want[kind])); err != nil || string(got) != "synthetic "+source {
						t.Fatalf("%s content = %q, err = %v", kind, got, err)
					}
				}
				second, err := BuildOrganizationPlan(root, local, remote, options)
				if err != nil || len(second.Moves) != 0 || len(second.Conflicts) != 0 {
					t.Fatalf("second plan = %#v, err = %v", second, err)
				}
			})
		}
	}
}

func TestTrailingDotCollisionsPreserveFiles(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		for _, safe := range []bool{false, true} {
			t.Run("occupied-"+strconv.FormatBool(occupied)+"/safe-"+strconv.FormatBool(safe), func(t *testing.T) {
				root := t.TempDir()
				incoming := filepath.Join(root, "in")
				mustMkdir(t, incoming)
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{}}
				remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}
				contents := map[string]string{}
				for _, file := range []struct{ name, id, title string }{
					{"first.nsp", "0100000000010000", "Game..."},
					{"second.nsp", "0100000000020000", "Game."},
					{"third.nsp", "0100000000030000", "Other..."},
				} {
					if occupied && file.name == "second.nsp" {
						continue
					}
					path := filepath.Join(incoming, file.name)
					writeFixture(t, path, file.name)
					contents[path] = file.name
					key := file.id[:13]
					local.TitlesMap[key] = &db.SwitchGameFiles{BaseExist: true, File: db.SwitchFileInfo{
						ExtendedInfo: extendedInfoForPath(t, path), Metadata: metadataForID(file.id, 0),
					}}
					remote.TitlesMap[key] = &db.SwitchTitle{Attributes: db.TitleAttributes{Id: file.id, Name: file.title}}
				}
				destination := filepath.Join(root, "Game", "Game.nsp")
				if occupied {
					mustMkdir(t, filepath.Dir(destination))
					writeFixture(t, destination, "existing destination")
					contents[destination] = "existing destination"
				}
				old := filepath.Join(root, "old.nsp")
				writeFixture(t, old, "cleanup candidate")
				contents[old] = "cleanup candidate"
				candidate := db.CleanupCandidate{File: extendedInfoForPath(t, old), Replacement: extendedInfoForPath(t, filepath.Join(incoming, "first.nsp"))}
				local.CleanupCandidates = []db.CleanupCandidate{candidate}
				options := settings.OrganizeOptions{
					RenameFiles: true, FileNameTemplate: "{TITLE_NAME}", CreateFolderPerGame: true,
					FolderNameTemplate: "{TITLE_NAME}", AllowUnicodeFileNames: !safe, DeleteOldUpdateFiles: true, DeleteEmptyFolders: true,
				}
				plan, err := BuildOrganizationPlan(root, local, remote, options)
				if err != nil || len(plan.Conflicts) == 0 {
					t.Fatalf("plan = %#v, err = %v; want conflict", plan, err)
				}
				for _, conflict := range plan.Conflicts {
					if conflict.Destination != destination {
						t.Fatalf("conflict destination = %q, want %q", conflict.Destination, destination)
					}
				}
				if err := ExecuteOrganizationPlan(plan, local, root, options, nil); err == nil {
					t.Fatal("execution accepted a trailing-dot collision")
				}
				for path, want := range contents {
					if got, err := os.ReadFile(path); err != nil || string(got) != want {
						t.Fatalf("file %q changed: content = %q, err = %v", path, got, err)
					}
				}
				if len(local.CleanupCandidates) != 1 || local.CleanupCandidates[0] != candidate {
					t.Fatal("preflight changed cleanup candidates")
				}
				assertNotExists(t, filepath.Join(root, "Other"))
				if !occupied {
					assertNotExists(t, filepath.Join(root, "Game"))
				}
			})
		}
	}
}

func TestEmptySanitizedOrganizationNames(t *testing.T) {
	for _, kind := range []string{"base", "update", "dlc", "split"} {
		for _, folders := range []bool{false, true} {
			if kind == "split" && !folders {
				continue // Split part filenames are never generated.
			}
			for _, safe := range []bool{false, true} {
				t.Run(kind+"/folders-"+strconv.FormatBool(folders)+"/safe-"+strconv.FormatBool(safe), func(t *testing.T) {
					root := t.TempDir()
					const baseID, key = "0100E95004039000", "0100e95004039"
					id := baseID
					name := "original.nsp"
					if kind == "split" {
						name = "parts.00"
					}
					if kind == "update" {
						id = "0100E95004039800"
					} else if kind == "dlc" {
						id = "0100E9500403A001"
					}
					path := filepath.Join(root, name)
					writeFixture(t, path, "synthetic source")
					file := db.SwitchFileInfo{ExtendedInfo: extendedInfoForPath(t, path), Metadata: metadataForID(id, 0)}
					game := &db.SwitchGameFiles{}
					switch kind {
					case "base", "split":
						game.BaseExist, game.File, game.IsSplit = true, file, kind == "split"
					case "update":
						game.Updates = map[int]db.SwitchFileInfo{0: file}
					case "dlc":
						game.Dlc = map[string]db.SwitchFileInfo{id: file}
					}
					local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: game}}
					remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
						key: {Attributes: db.TitleAttributes{Id: baseID, Name: "..."}},
					}}
					options := settings.OrganizeOptions{
						RenameFiles: true, FileNameTemplate: "{TITLE_NAME}", CreateFolderPerGame: folders,
						FolderNameTemplate: "{TITLE_NAME}", AllowUnicodeFileNames: !safe, ProcessWhenMissingBaseGame: true,
					}
					want := "empty file stem"
					if folders {
						want = "empty game folder name"
						options.FileNameTemplate = "{TITLE_ID}"
					}
					if _, err := BuildOrganizationPreview(root, options, local, remote); err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "{TITLE_NAME}") {
						t.Fatalf("preview error = %v, want contextual %q error", err, want)
					}
					plan, err := BuildOrganizationPlan(root, local, remote, options)
					if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "{TITLE_NAME}") {
						t.Fatalf("plan error = %v, want contextual %q error", err, want)
					}
					if len(plan.Moves) != 0 {
						t.Fatalf("invalid name produced moves: %#v", plan)
					}
					if got, err := os.ReadFile(path); err != nil || string(got) != "synthetic source" {
						t.Fatalf("source changed: content = %q, err = %v", got, err)
					}
					entries, err := os.ReadDir(root)
					if err != nil || len(entries) != 1 || entries[0].Name() != name {
						t.Fatalf("invalid name changed the directory: entries = %#v, err = %v", entries, err)
					}
				})
			}
		}
	}
}
