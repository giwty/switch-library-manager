package process

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

func TestBuildOrganizationPlanFindsDuplicateDestinationsBeforeMoving(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	firstPath := filepath.Join(source, "first.nsp")
	secondPath := filepath.Join(source, "second.nsp")
	writeFixture(t, firstPath, "first DLC")
	writeFixture(t, secondPath, "second DLC")

	firstID := "0100E9500403A001"
	secondID := "0100E9500403A002"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				firstID:  {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "first.nsp"}, Metadata: metadataForID(firstID, 1)},
				secondID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "second.nsp"}, Metadata: metadataForID(secondID, 1)},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME}",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v, want both colliding DLC files", plan.Conflicts)
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("planned moves = %d, want 2 before validation", len(plan.Moves))
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted conflicting plan")
	} else {
		var conflictErr *OrganizationConflictsError
		if !errors.As(err, &conflictErr) || len(conflictErr.Conflicts) != 2 {
			t.Fatalf("ExecuteOrganizationPlan() error = %v, want conflict error", err)
		}
	}
	assertExists(t, firstPath)
	assertExists(t, secondPath)
	assertNotExists(t, filepath.Join(source, "Game.nsp"))
}

func TestOrganizationAndPreviewPreferFilenameOverJapaneseTitle(t *testing.T) {
	root := t.TempDir()
	sourceFolder := filepath.Join(root, "incoming")
	mustMkdir(t, sourceFolder)
	writeFixture(t, filepath.Join(sourceFolder, "test.nsp"), "synthetic base")

	const (
		baseID = "010044901C5C2000"
		key    = "010044901c5c2"
		name   = "カプコン ファイティング コレクション2"
	)
	localGame := &db.SwitchGameFiles{
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: "test.nsp"},
			Metadata:     contentMetadata(baseID, 0, ""),
		},
	}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{key: localGame}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		key: {Attributes: db.TitleAttributes{Id: baseID, Name: name}},
	}}
	options := settings.OrganizeOptions{
		CreateFolderPerGame: true,
		FolderNameTemplate:  "{TITLE_NAME}",
		RenameFiles:         true,
		FileNameTemplate:    "{TITLE_NAME}",
	}
	want := filepath.Join("test", "test.nsp")

	preview, err := BuildOrganizationPreview(root, options, local, remote)
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	gamePreviewPath := ""
	for _, entry := range preview {
		if entry.Kind == "game" {
			gamePreviewPath = entry.Path
		}
	}
	if gamePreviewPath != want {
		t.Fatalf("game preview path = %q, want %q (all entries: %#v)", gamePreviewPath, want, preview)
	}

	plan, err := BuildOrganizationPlan(root, local, remote, options)
	if err != nil {
		t.Fatalf("build organization plan: %v", err)
	}
	if len(plan.Conflicts) != 0 || len(plan.Moves) != 1 || plan.Moves[0].Destination != filepath.Join(root, want) {
		t.Fatalf("organization plan = %#v, want one move to %q", plan, filepath.Join(root, want))
	}
}

func TestDLCNameTrimmingMatchesPreviewAndRemainsStable(t *testing.T) {
	for _, test := range []struct {
		name string
		safe bool
	}{
		{name: "names unchanged", safe: false},
		{name: "safe names", safe: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			source := filepath.Join(base, "incoming")
			mustMkdir(t, source)
			sourcePath := filepath.Join(source, "Aquarium Land ( Puppy)[0100BAE01C1FB001][v0].nsz")
			writeFixture(t, sourcePath, "synthetic DLC")

			dlcID := "0100BAE01C1FB001"
			gameKey := "0100bae01c1fa"
			local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
				gameKey: {
					Dlc: map[string]db.SwitchFileInfo{
						dlcID: {
							ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: filepath.Base(sourcePath)},
							Metadata:     metadataForID(dlcID, 0),
						},
					},
				},
			}}
			remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
				gameKey: {
					Attributes: db.TitleAttributes{Name: "Aquarium Land"},
					Dlc:        map[string]db.TitleAttributes{dlcID: {Name: "Aquarium Land: Puppy"}},
				},
			}}
			options := settings.OrganizeOptions{
				RenameFiles:                true,
				FileNameTemplate:           "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]",
				ProcessWhenMissingBaseGame: true,
				AllowUnicodeFileNames:      !test.safe,
			}

			preview, err := BuildOrganizationPreview(base, options, local, remote)
			if err != nil {
				t.Fatal(err)
			}
			var previewPath string
			for _, entry := range preview {
				if entry.Kind == "dlc" {
					previewPath = filepath.Join(base, entry.Path)
					break
				}
			}
			want := filepath.Join(source, "Aquarium Land (Puppy)[0100BAE01C1FB001][v0].nsz")
			if previewPath != want {
				t.Fatalf("DLC preview path = %q, want %q", previewPath, want)
			}

			plan, err := BuildOrganizationPlan(base, local, remote, options)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Conflicts) != 0 || len(plan.Moves) != 1 || plan.Moves[0].Destination != previewPath {
				t.Fatalf("plan = %#v, want one move to preview path %q", plan, previewPath)
			}
			if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err != nil {
				t.Fatalf("execute first organization: %v", err)
			}
			assertMoved(t, sourcePath, previewPath, "DLC after colon-trimmed organization")
			if content, err := os.ReadFile(previewPath); err != nil || string(content) != "synthetic DLC" {
				t.Fatalf("renamed DLC content = %q, err = %v", content, err)
			}

			secondPlan, err := BuildOrganizationPlan(base, local, remote, options)
			if err != nil {
				t.Fatal(err)
			}
			if len(secondPlan.Conflicts) != 0 || len(secondPlan.Moves) != 0 {
				t.Fatalf("second organization plan = %#v, want no moves or conflicts", secondPlan)
			}
		})
	}
}

func TestDLCNameTrimmingCollisionPreservesFiles(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	firstPath := filepath.Join(source, "first.nsp")
	secondPath := filepath.Join(source, "second.nsp")
	destinationPath := filepath.Join(source, "Game (Expansion).nsp")
	writeFixture(t, firstPath, "first DLC")
	writeFixture(t, secondPath, "second DLC")

	firstID := "0100E9500403A001"
	secondID := "0100E9500403A002"
	gameKey := "0100e95004039"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		gameKey: {
			Dlc: map[string]db.SwitchFileInfo{
				firstID:  {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: filepath.Base(firstPath)}, Metadata: metadataForID(firstID, 1)},
				secondID: {ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: filepath.Base(secondPath)}, Metadata: metadataForID(secondID, 1)},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		gameKey: {
			Attributes: db.TitleAttributes{Name: "Game"},
			Dlc: map[string]db.TitleAttributes{
				firstID:  {Name: "Game: Expansion"},
				secondID: {Name: "Game - Expansion"},
			},
		},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} ({DLC_NAME})",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) == 0 {
		t.Fatal("plan accepted DLC names that collide after edge trimming")
	}
	duplicateDestination := false
	for _, conflict := range plan.Conflicts {
		if conflict.Reason == "multiple files have the same destination" {
			duplicateDestination = true
			break
		}
	}
	if !duplicateDestination {
		t.Fatalf("plan did not report the colon/dash DLC collision: %#v", plan.Conflicts)
	}
	writeFixture(t, destinationPath, "existing file")
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("execution accepted a plan with a trimmed-name collision")
	}
	for path, want := range map[string]string{firstPath: "first DLC", secondPath: "second DLC"} {
		if content, err := os.ReadFile(path); err != nil || string(content) != want {
			t.Errorf("source %q changed: content=%q err=%v", path, content, err)
		}
	}
	if content, err := os.ReadFile(destinationPath); err != nil || string(content) != "existing file" {
		t.Fatalf("existing destination changed: content=%q err=%v", content, err)
	}
}

func TestBuildOrganizationPlanReportsOccupiedDestinationWithoutOverwrite(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "original.nsp")
	destinationPath := filepath.Join(source, "Game.nsp")
	writeFixture(t, sourcePath, "source content")
	writeFixture(t, destinationPath, "existing destination")
	file := db.ExtendedFileInfo{BaseFolder: source, FileName: "original.nsp"}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: file, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}"}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Reason != "destination already exists" {
		t.Fatalf("conflicts = %#v, want occupied destination", plan.Conflicts)
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted occupied destination")
	}
	assertExists(t, sourcePath)
	content, err := os.ReadFile(destinationPath)
	if err != nil || string(content) != "existing destination" {
		t.Fatalf("existing destination changed: content=%q err=%v", content, err)
	}
}

func TestBuildOrganizationPlanReportsBlockedDestinationParent(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "original.nsp")
	writeFixture(t, sourcePath, "source content")
	blockedParent := filepath.Join(base, "occupied")
	writeFixture(t, blockedParent, "not a folder")
	file := db.ExtendedFileInfo{BaseFolder: source, FileName: "original.nsp"}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: file, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "occupied"}},
	}}
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].OtherSource != blockedParent {
		t.Fatalf("conflicts = %#v, want blocked parent %q", plan.Conflicts, blockedParent)
	}
	assertExists(t, sourcePath)
}

func TestAddOrganizationMovePreflightOutcomes(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	blockedParent := filepath.Join(base, "blocked")
	writeFixture(t, blockedParent, "not a directory")
	existingDestination := filepath.Join(base, "existing.nsp")
	writeFixture(t, existingDestination, "existing")

	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, source); err != nil || len(plan.Moves) != 0 {
		t.Fatalf("same-path move: moves=%#v err=%v", plan.Moves, err)
	}
	seen := map[string]struct{}{}
	if err := addOrganizationMove(&plan, seen, source, filepath.Join(base, "target.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, seen, source, filepath.Join(base, "another.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 1 {
		t.Fatalf("duplicate source was planned more than once: %#v", plan.Moves)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, filepath.Join(base, "missing.nsp"), filepath.Join(base, "missing-target.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, base, filepath.Join(base, "directory-target")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(blockedParent, "child.nsp")); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, existingDestination); err != nil {
		t.Fatal(err)
	}
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(base, "invalid\x00parent", "target.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 5 {
		t.Fatalf("preflight conflicts = %#v, want missing source, directory source, blocked parent, occupied destination, and invalid parent", plan.Conflicts)
	}
}

func TestAddOrganizationMoveSkipsHardlinkAlias(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	destination := filepath.Join(base, "destination.nsp")
	writeFixture(t, source, "same inode")
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hard links are unavailable in this test environment: %v", err)
	}
	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, destination); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("hard-link alias produced a move or conflict: %#v", plan)
	}
}

func TestOrganizationConflictsErrorText(t *testing.T) {
	err := (&OrganizationConflictsError{Conflicts: []OrganizationConflict{{}, {}}}).Error()
	if err != "organization canceled: 2 destination conflict(s); see Issues" {
		t.Fatalf("conflict error text = %q", err)
	}
}

func TestUniqueOrganizationConflictsRemovesDuplicates(t *testing.T) {
	conflict := OrganizationConflict{Source: "source", Destination: "destination", Reason: "occupied"}
	unique := uniqueOrganizationConflicts([]OrganizationConflict{conflict, conflict})
	if len(unique) != 1 || unique[0] != conflict {
		t.Fatalf("unique conflicts = %#v", unique)
	}
}

func TestBuildOrganizationPlanRejectsInvalidOptions(t *testing.T) {
	_, err := BuildOrganizationPlan(t.TempDir(), &db.LocalSwitchFilesDB{}, nil, settings.OrganizeOptions{
		RenameFiles: true,
	})
	if err == nil {
		t.Fatal("BuildOrganizationPlan() accepted an empty rename template")
	}
	if _, err := BuildOrganizationPlan(t.TempDir(), nil, nil, settings.OrganizeOptions{}); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a missing local scan")
	}
}

func TestBuildOrganizationPlanUsesFilenameMetadataWithoutRemoteTitle(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)
	fileName := "Game [0100E95004039000][v0].nsp"
	writeFixture(t, filepath.Join(sourceFolder, fileName), "base")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: fileName}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_ID}"}
	plan, err := BuildOrganizationPlan(base, local, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 0 || len(plan.Moves) != 1 || filepath.Base(plan.Moves[0].Destination) != "0100E95004039000.nsp" {
		t.Fatalf("filename metadata plan = %#v", plan)
	}
}

func TestBuildOrganizationPlanRepairsRepeatedSuffixesOnce(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)
	fileName := "Capcom Fighting Collection 2 (v0 v1.0.0).nsp"
	source := filepath.Join(sourceFolder, fileName)
	writeFixture(t, source, "synthetic base")

	titleID := "010044901C5C2000"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"010044901c5c2": {
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: fileName},
				Metadata:     contentMetadata(titleID, 0, "1.0.0"),
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"010044901c5c2": {Attributes: db.TitleAttributes{Id: titleID, Name: "カプコン ファイティング コレクション2"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:      true,
		FileNameTemplate: "{TITLE_NAME} (v{VERSION}) (v{VERSION_TXT})",
	}
	wantPath := filepath.Join(sourceFolder, "Capcom Fighting Collection 2 (v0) (v1.0.0).nsp")

	firstPlan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPlan.Conflicts) != 0 || len(firstPlan.Moves) != 1 || firstPlan.Moves[0].Destination != wantPath {
		t.Fatalf("first organization plan = %#v, want one move to %q", firstPlan, wantPath)
	}
	if err := ExecuteOrganizationPlan(firstPlan, local, base, options, nil); err != nil {
		t.Fatalf("execute first organization plan: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("original file still exists after repair, stat error = %v", err)
	}
	assertExists(t, wantPath)

	secondPlan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPlan.Conflicts) != 0 || len(secondPlan.Moves) != 0 {
		t.Fatalf("second organization plan = %#v, want no further moves", secondPlan)
	}
}

func TestBuildOrganizationPlanKeepsFilesWhenSuffixRepairCollides(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)

	const titleName = "カプコン ファイティング コレクション2"
	files := []struct {
		id       string
		fileName string
	}{
		{id: "010044901C5C2000", fileName: "Capcom Fighting Collection 2 (v0 v1.0.0).nsp"},
		{id: "010044901C5C3000", fileName: "Capcom Fighting Collection 2 (v0) (v1.0.0).nsp"},
	}
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{}}
	for _, file := range files {
		source := filepath.Join(sourceFolder, file.fileName)
		writeFixture(t, source, file.id)
		key := strings.ToLower(file.id[:len(file.id)-3])
		local.TitlesMap[key] = &db.SwitchGameFiles{
			BaseExist: true,
			File: db.SwitchFileInfo{
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: file.fileName},
				Metadata:     contentMetadata(file.id, 0, "1.0.0"),
			},
		}
		remote.TitlesMap[key] = &db.SwitchTitle{
			Attributes: db.TitleAttributes{Id: file.id, Name: titleName},
		}
	}
	options := settings.OrganizeOptions{
		RenameFiles:         true,
		CreateFolderPerGame: true,
		FolderNameTemplate:  "{TITLE_NAME}",
		FileNameTemplate:    "{TITLE_NAME} (v{VERSION}) (v{VERSION_TXT})",
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 2 || len(plan.Moves) != 2 {
		t.Fatalf("collision plan = %#v, want both sources blocked", plan)
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted colliding repaired names")
	}
	for _, file := range files {
		path := filepath.Join(sourceFolder, file.fileName)
		content, err := os.ReadFile(path)
		if err != nil || string(content) != file.id {
			t.Fatalf("source %q changed: content=%q err=%v", path, content, err)
		}
	}
	assertNotExists(t, filepath.Join(base, "Capcom Fighting Collection 2", "Capcom Fighting Collection 2 (v0) (v1.0.0).nsp"))
}

func TestBuildOrganizationPlanUsesV0DLCDisplayVersionFallback(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	writeFixture(t, filepath.Join(source, "dlc.nsp"), "synthetic DLC")
	dlcID := "0100E9500403A001"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {
					ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: "dlc.nsp"},
					Metadata:     metadataForID(dlcID, 0),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} [v{VERSION}][v{VERSION_TXT}]",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(source, "Game [v0][v1.0.0].nsp")
	if len(plan.Moves) != 1 || plan.Moves[0].Destination != want {
		t.Fatalf("moves = %#v, want destination %q", plan.Moves, want)
	}
}

func TestBuildOrganizationPlanBlocksOccupiedV0DLCFallbackDestination(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "incoming")
	mustMkdir(t, source)
	sourcePath := filepath.Join(source, "dlc.nsp")
	destinationPath := filepath.Join(source, "Game [v0][v1.0.0].nsp")
	writeFixture(t, sourcePath, "synthetic DLC")
	writeFixture(t, destinationPath, "existing destination")
	dlcID := "0100E9500403A001"
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100e95004039": {
			Dlc: map[string]db.SwitchFileInfo{
				dlcID: {
					ExtendedInfo: db.ExtendedFileInfo{BaseFolder: source, FileName: filepath.Base(sourcePath)},
					Metadata:     metadataForID(dlcID, 0),
				},
			},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100e95004039": {Attributes: db.TitleAttributes{Name: "Game"}},
	}}
	options := settings.OrganizeOptions{
		RenameFiles:                true,
		FileNameTemplate:           "{TITLE_NAME} [v{VERSION}][v{VERSION_TXT}]",
		ProcessWhenMissingBaseGame: true,
	}

	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Destination != destinationPath {
		t.Fatalf("conflicts = %#v, want occupied destination %q", plan.Conflicts, destinationPath)
	}
	if err := ExecuteOrganizationPlan(plan, local, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted an occupied v0 DLC destination")
	}
	if content, err := os.ReadFile(sourcePath); err != nil || string(content) != "synthetic DLC" {
		t.Fatalf("source changed during planning: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(destinationPath); err != nil || string(content) != "existing destination" {
		t.Fatalf("destination changed during planning: content=%q err=%v", content, err)
	}
}

func TestBuildOrganizationPlanRejectsInvalidSplitModel(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "split")
	mustMkdir(t, sourceFolder)
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			IsSplit:   true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: "game.nsp"}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	options := settings.OrganizeOptions{CreateFolderPerGame: true, FolderNameTemplate: "{TITLE_NAME}"}
	if _, err := BuildOrganizationPlan(base, local, nil, options); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a split record without a part-00 filename")
	}
	local.TitlesMap["0100E95004039"].File.ExtendedInfo.BaseFolder = filepath.Join(base, "missing")
	local.TitlesMap["0100E95004039"].File.ExtendedInfo.FileName = "game.00"
	if _, err := BuildOrganizationPlan(base, local, nil, options); err == nil {
		t.Fatal("BuildOrganizationPlan() accepted a missing split directory")
	}
}

func TestAddSplitMovesFiltersPartsAndNonnumericFiles(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "split")
	destination := filepath.Join(base, "organized")
	mustMkdir(t, source)
	for name, content := range map[string]string{
		"game.00":  "part 0",
		"game.01":  "part 1",
		"game.xx":  "not a part",
		"other.02": "other file",
	} {
		writeFixture(t, filepath.Join(source, name), content)
	}
	var plan OrganizationPlan
	if err := addSplitMoves(&plan, map[string]struct{}{}, "game.00", source, destination); err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("split moves = %#v, want only parts 00 and 01", plan.Moves)
	}
	if err := addSplitMoves(&plan, map[string]struct{}{}, "game.00", filepath.Join(base, "missing"), destination); err == nil {
		t.Fatal("addSplitMoves() accepted a missing split directory")
	}
}

func TestValidateOrganizationPlanCatchesDestinationThatIsAnotherSource(t *testing.T) {
	base := t.TempDir()
	first := filepath.Join(base, "first.nsp")
	second := filepath.Join(base, "second.nsp")
	third := filepath.Join(base, "third.nsp")
	for _, path := range []string{first, second} {
		writeFixture(t, path, "fixture")
	}
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	plan := OrganizationPlan{Moves: []OrganizationMove{
		{Source: first, Destination: second, SourceSize: firstInfo.Size(), SourceStamp: firstInfo},
		{Source: second, Destination: third, SourceSize: secondInfo.Size(), SourceStamp: secondInfo},
	}}
	validateOrganizationPlan(&plan)
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Reason != "destination belongs to another planned source" {
		t.Fatalf("planned-source conflict = %#v", plan.Conflicts)
	}
}

func TestAddOrganizationMoveReportsInvalidDestinationPath(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	var plan OrganizationPlan
	if err := addOrganizationMove(&plan, map[string]struct{}{}, source, filepath.Join(base, "invalid\x00destination.nsp")); err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || !strings.Contains(plan.Conflicts[0].Reason, "cannot inspect destination") {
		t.Fatalf("invalid destination conflict = %#v", plan.Conflicts)
	}
}

func TestExecuteOrganizationPlanRechecksSourceAndLateDestination(t *testing.T) {
	base := t.TempDir()
	sourceFolder := filepath.Join(base, "incoming")
	mustMkdir(t, sourceFolder)
	source := filepath.Join(sourceFolder, "original.nsp")
	writeFixture(t, source, "before")
	local := &db.LocalSwitchFilesDB{TitlesMap: map[string]*db.SwitchGameFiles{
		"0100E95004039": {
			BaseExist: true,
			File:      db.SwitchFileInfo{ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sourceFolder, FileName: "original.nsp"}, Metadata: metadataForID("0100E95004039000", 0)},
		},
	}}
	remote := &db.SwitchTitlesDB{TitlesMap: map[string]*db.SwitchTitle{
		"0100E95004039": {Attributes: db.TitleAttributes{Id: "0100E95004039000", Name: "Game"}},
	}}
	options := settings.OrganizeOptions{RenameFiles: true, FileNameTemplate: "{TITLE_NAME}"}
	plan, err := BuildOrganizationPlan(base, local, remote, options)
	if err != nil || len(plan.Conflicts) != 0 {
		t.Fatalf("BuildOrganizationPlan() = %#v, %v", plan, err)
	}
	writeFixture(t, source, "source changed after preflight")
	if err := ExecuteOrganizationPlan(plan, nil, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() moved a source changed after preflight")
	}
	assertExists(t, source)
	if err := ExecuteOrganizationPlan(OrganizationPlan{}, nil, base, options, nil); err != nil {
		t.Fatalf("ExecuteOrganizationPlan(empty) = %v", err)
	}

	writeFixture(t, source, "before second plan")
	plan, err = BuildOrganizationPlan(base, local, remote, options)
	if err != nil || len(plan.Conflicts) != 0 {
		t.Fatalf("second BuildOrganizationPlan() = %#v, %v", plan, err)
	}
	destination := filepath.Join(sourceFolder, "Game.nsp")
	writeFixture(t, destination, "appeared after preflight")
	if err := ExecuteOrganizationPlan(plan, nil, base, options, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() overwrote a late destination")
	}
	assertExists(t, source)
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "appeared after preflight" {
		t.Fatalf("late destination changed: content=%q err=%v", content, err)
	}
}

func TestExecuteOrganizationPlanReportsMissingSourceBlockedDirectoryAndCleanupErrors(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "missing.nsp")
	if err := ExecuteOrganizationPlan(OrganizationPlan{Moves: []OrganizationMove{{
		Source: missing, Destination: filepath.Join(base, "target.nsp"),
	}}}, nil, base, settings.OrganizeOptions{}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() accepted a disappeared source")
	}

	source := filepath.Join(base, "source.nsp")
	writeFixture(t, source, "source")
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	blockedParent := filepath.Join(base, "blocked")
	writeFixture(t, blockedParent, "not a directory")
	blockedPlan := OrganizationPlan{Moves: []OrganizationMove{{
		Source: source, Destination: filepath.Join(blockedParent, "target.nsp"), SourceSize: info.Size(), SourceStamp: info,
	}}}
	if err := ExecuteOrganizationPlan(blockedPlan, nil, base, settings.OrganizeOptions{}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() created a child beneath a file")
	}
	assertExists(t, source)

	cleanupSource := filepath.Join(base, "cleanup-source.nsp")
	cleanupDestination := filepath.Join(base, "cleanup-destination.nsp")
	writeFixture(t, cleanupSource, "cleanup")
	cleanupInfo, err := os.Stat(cleanupSource)
	if err != nil {
		t.Fatal(err)
	}
	cleanupPlan := OrganizationPlan{Moves: []OrganizationMove{{
		Source: cleanupSource, Destination: cleanupDestination, SourceSize: cleanupInfo.Size(), SourceStamp: cleanupInfo,
	}}}
	if err := ExecuteOrganizationPlan(cleanupPlan, nil, filepath.Join(base, "missing-cleanup-root"), settings.OrganizeOptions{DeleteEmptyFolders: true}, nil); err == nil {
		t.Fatal("ExecuteOrganizationPlan() ignored empty-folder cleanup failure")
	}
	assertExists(t, cleanupDestination)
}

func TestUpdateLocalFilePathsReportsMissingMovedFile(t *testing.T) {
	missingDestination := filepath.Join(t.TempDir(), "missing.nsp")
	if err := updateLocalFilePaths(&db.LocalSwitchFilesDB{}, "source.nsp", missingDestination); err == nil {
		t.Fatal("updateLocalFilePaths() accepted a missing moved file")
	}
}

func TestDeleteEmptyFoldersPreservesRootAndRemovesEmptyChildren(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	child := filepath.Join(root, "empty", "nested")
	mustMkdir(t, child)
	if err := deleteEmptyFolders(root); err != nil {
		t.Fatal(err)
	}
	assertExists(t, root)
	assertNotExists(t, filepath.Join(root, "empty"))
}
