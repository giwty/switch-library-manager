package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

func TestUnicodeNamesSharedAcrossViewsAndOrganization(t *testing.T) {
	for _, catalogName := range []string{"日本語: デモ?", "한국어: 게임?", "中文: 游戏?"} {
		for _, allowUnicode := range []bool{false, true} {
			t.Run(catalogName+"/unicode="+strconv.FormatBool(allowUnicode), func(t *testing.T) {
				root := t.TempDir()
				incoming := filepath.Join(root, "incoming")
				if err := os.Mkdir(incoming, 0755); err != nil {
					t.Fatal(err)
				}
				makeFile := func(name, id string) db.SwitchFileInfo {
					t.Helper()
					if err := os.WriteFile(filepath.Join(incoming, name), []byte("synthetic fixture"), 0644); err != nil {
						t.Fatal(err)
					}
					return db.SwitchFileInfo{
						ExtendedInfo: db.ExtendedFileInfo{BaseFolder: incoming, FileName: name},
						Metadata: &switchfs.ContentMetaAttributes{TitleId: id, Ncap: &switchfs.Nacp{
							TitleName: map[string]switchfs.NacpTitle{"AmericanEnglish": {Title: "English fallback"}},
						}},
					}
				}
				game := &db.SwitchGameFiles{
					BaseExist: true,
					File:      makeFile("base.nsp", detailsBaseID),
					Updates:   map[int]db.SwitchFileInfo{1: makeFile("update.nsp", "0100000000010800")},
					Dlc:       map[string]db.SwitchFileInfo{detailsDLCID: makeFile("dlc.nsp", detailsDLCID)},
				}
				local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{detailsPrefix: game}}
				remoteTitle := &db.SwitchTitle{
					Attributes: db.TitleAttributes{Id: detailsBaseID, Name: catalogName},
					Dlc:        map[string]db.TitleAttributes{detailsDLCID: {Id: detailsDLCID, Name: catalogName + " - 拡張 콘텐츠"}},
				}
				remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{detailsPrefix: remoteTitle}}
				options := settings.OrganizeOptions{
					AllowUnicodeFileNames: allowUnicode,
					CreateFolderPerGame:   true,
					FolderNameTemplate:    "{TITLE_NAME}",
					RenameFiles:           true,
					FileNameTemplate:      "{TITLE_NAME} ({DLC_NAME})[{TYPE}]",
					IncludeBaseType:       true,
					UpdatesFolder:         "updates",
					DlcFolder:             "dlc",
				}
				wantTitle := "English fallback"
				if allowUnicode {
					wantTitle = catalogName
				}
				library := buildLocalLibraryData(local, remote, allowUnicode)
				details, err := buildGameDetails(detailsBaseID, local, remote, settings.MissingContentSettings{}, allowUnicode)
				if err != nil {
					t.Fatal(err)
				}
				if len(library.LibraryData) != 1 || library.LibraryData[0].Name != wantTitle || details.Name != wantTitle || details.Attributes.Name != wantTitle {
					t.Fatalf("inconsistent displayed titles: library=%#v details=%#v, want %q", library.LibraryData, details, wantTitle)
				}
				preview, err := process.BuildOrganizationPreview(root, options, local, remote)
				if err != nil {
					t.Fatal(err)
				}
				plan, err := process.BuildOrganizationPlan(root, local, remote, options)
				if err != nil {
					t.Fatal(err)
				}
				if len(preview) != 3 || len(plan.Moves) != 3 || len(plan.Conflicts) != 0 {
					t.Fatalf("unexpected preview/plan: %#v, %#v", preview, plan)
				}
				wantFolder := strings.NewReplacer(":", "", "?", "").Replace(wantTitle)
				for _, entry := range preview {
					if !strings.HasPrefix(entry.Path, wantFolder+string(filepath.Separator)) {
						t.Fatalf("preview title does not match displayed title: %q", entry.Path)
					}
					found := false
					for _, move := range plan.Moves {
						found = found || move.Destination == filepath.Join(root, entry.Path)
					}
					if !found {
						t.Fatalf("preview destination missing from plan: %q", entry.Path)
					}
					if allowUnicode && entry.Kind == "dlc" && !strings.Contains(entry.Path, "(拡張 콘텐츠)") {
						t.Fatalf("Unicode DLC name was filtered: %q", entry.Path)
					}
				}
				if err := process.ExecuteOrganizationPlan(plan, local, root, options, nil); err != nil {
					t.Fatal(err)
				}
				for _, move := range plan.Moves {
					if _, err := os.Stat(move.Destination); err != nil {
						t.Fatalf("organized destination %q: %v", move.Destination, err)
					}
					if _, err := os.Stat(move.Source); !os.IsNotExist(err) {
						t.Fatalf("source still exists after organizing: %q, %v", move.Source, err)
					}
				}
				if remoteTitle.Attributes.Name != catalogName {
					t.Fatalf("catalog title was mutated: %q", remoteTitle.Attributes.Name)
				}
			})
		}
	}
}
