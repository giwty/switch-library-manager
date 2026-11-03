package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

// OrganizationMove describes one physical file move after template expansion.
type OrganizationMove struct {
	Source      string
	Destination string
	SourceSize  int64
	SourceStamp os.FileInfo
}

// OrganizationConflict describes a source whose planned destination cannot be
// used without replacing or colliding with another file.
type OrganizationConflict struct {
	Source      string
	Destination string
	OtherSource string
	Reason      string
}

// OrganizationPlan contains every move and preflight issue found for a scan.
type OrganizationPlan struct {
	Moves     []OrganizationMove
	Conflicts []OrganizationConflict
}

// OrganizationConflictsError marks a plan rejected before any filesystem
// changes. Callers can publish its conflicts in their existing Issues view.
type OrganizationConflictsError struct {
	Conflicts []OrganizationConflict
}

func (e *OrganizationConflictsError) Error() string {
	return fmt.Sprintf("organization canceled: %d destination conflict(s); see Issues", len(e.Conflicts))
}

// BuildOrganizationPlan computes and validates all destinations without
// creating directories or changing library files. Files outside baseFolder,
// such as those in scan folders, are left untouched unless
// MoveScanFilesToLibrary imports them into baseFolder.
func BuildOrganizationPlan(
	baseFolder string,
	localDB *db.LocalSwitchFilesDB,
	titlesDB *db.SwitchTitlesDB,
	options settings.OrganizeOptions,
) (OrganizationPlan, error) {
	if localDB == nil {
		return OrganizationPlan{}, fmt.Errorf("local library has not been scanned")
	}
	if !IsOptionsValid(options) {
		return OrganizationPlan{}, fmt.Errorf("organization options are invalid")
	}

	plan := OrganizationPlan{Moves: []OrganizationMove{}, Conflicts: []OrganizationConflict{}}
	keys := make([]string, 0, len(localDB.TitlesMap))
	for key := range localDB.TitlesMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seenSources := map[string]struct{}{}

	for _, key := range keys {
		game := localDB.TitlesMap[key]
		if game == nil || (!game.BaseExist && !options.ProcessWhenMissingBaseGame) {
			continue
		}
		title := (*db.SwitchTitle)(nil)
		if titlesDB != nil {
			title = titlesDB.TitlesMap[key]
		}
		titleName := db.ResolveGameTitleName(title, game, options.AllowUnicodeFileNames)
		templateData := map[string]string{
			settings.TEMPLATE_TITLE_NAME:  titleName,
			settings.TEMPLATE_VERSION_TXT: "",
		}
		if title != nil {
			templateData[settings.TEMPLATE_TITLE_ID] = title.Attributes.Id
			templateData[settings.TEMPLATE_REGION] = title.Attributes.Region
		}
		if templateData[settings.TEMPLATE_TITLE_ID] == "" && game.File.Metadata != nil {
			templateData[settings.TEMPLATE_TITLE_ID] = game.File.Metadata.TitleId
		}
		setBaseFileVersionTemplateData(templateData, game)

		gameFolder := ""
		if options.CreateFolderPerGame {
			gameFolder = getFolderName(options, templateData)
		}

		if game.IsSplit && game.BaseExist {
			fileDestination, included, err := organizationDestinationFolder(baseFolder, gameFolder, game.File.ExtendedInfo.BaseFolder, options)
			if err != nil {
				return OrganizationPlan{}, fmt.Errorf("resolve split destination for %q: %w", key, err)
			}
			if !included {
				continue
			}
			if err := addSplitMoves(&plan, seenSources, game.File.ExtendedInfo.FileName, game.File.ExtendedInfo.BaseFolder, fileDestination); err != nil {
				return OrganizationPlan{}, err
			}
			continue
		}

		if game.BaseExist {
			templateData[settings.TEMPLATE_TYPE] = "BASE"
			templateData[settings.TEMPLATE_PACKAGE_CONTENTS] = packageContents(game)
			setFileSizeTemplateData(templateData, game.File.ExtendedInfo.Size)
			target, included, err := organizationTargetPath(baseFolder, gameFolder, game.File.ExtendedInfo.BaseFolder, game.File.ExtendedInfo.FileName, options, "base", templateData, 0)
			if err != nil {
				return OrganizationPlan{}, fmt.Errorf("resolve base destination for %q: %w", key, err)
			}
			if included {
				if err := addOrganizationMove(&plan, seenSources, filepath.Join(game.File.ExtendedInfo.BaseFolder, game.File.ExtendedInfo.FileName), target); err != nil {
					return OrganizationPlan{}, fmt.Errorf("plan base file for %q: %w", key, err)
				}
			}
		}
		templateData[settings.TEMPLATE_PACKAGE_CONTENTS] = ""

		updateVersions := make([]int, 0, len(game.Updates))
		for version := range game.Updates {
			updateVersions = append(updateVersions, version)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(updateVersions)))
		for _, version := range updateVersions {
			update := game.Updates[version]
			if game.BaseExist && samePhysicalFilePath(game.File.ExtendedInfo, update.ExtendedInfo) {
				continue
			}
			if update.Metadata != nil {
				templateData[settings.TEMPLATE_TITLE_ID] = update.Metadata.TitleId
				if update.Metadata.Ncap != nil {
					templateData[settings.TEMPLATE_VERSION_TXT] = update.Metadata.Ncap.DisplayVersion
				}
			}
			templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(version)
			templateData[settings.TEMPLATE_TYPE] = "UPD"
			setFileSizeTemplateData(templateData, update.ExtendedInfo.Size)
			target, included, err := organizationTargetPath(baseFolder, gameFolder, update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName, options, "update", templateData, 0)
			if err != nil {
				return OrganizationPlan{}, fmt.Errorf("resolve update destination for %q: %w", key, err)
			}
			if !included {
				continue
			}
			if err := addOrganizationMove(&plan, seenSources, filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName), target); err != nil {
				return OrganizationPlan{}, fmt.Errorf("plan update file for %q: %w", key, err)
			}
		}

		dlcIDs := make([]string, 0, len(game.Dlc))
		for id := range game.Dlc {
			dlcIDs = append(dlcIDs, id)
		}
		sort.Strings(dlcIDs)
		for _, id := range dlcIDs {
			dlc := game.Dlc[id]
			if game.BaseExist && samePhysicalFilePath(game.File.ExtendedInfo, dlc.ExtendedInfo) {
				continue
			}
			setDlcVersionTemplateData(templateData, dlc)
			templateData[settings.TEMPLATE_TYPE] = "DLC"
			templateData[settings.TEMPLATE_TITLE_ID] = id
			templateData[settings.TEMPLATE_DLC_NAME] = getDlcName(title, dlc)
			setFileSizeTemplateData(templateData, dlc.ExtendedInfo.Size)
			target, included, err := organizationTargetPath(baseFolder, gameFolder, dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName, options, "dlc", templateData, 0)
			if err != nil {
				return OrganizationPlan{}, fmt.Errorf("resolve DLC destination for %q: %w", key, err)
			}
			if !included {
				continue
			}
			if err := addOrganizationMove(&plan, seenSources, filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName), target); err != nil {
				return OrganizationPlan{}, fmt.Errorf("plan DLC file for %q: %w", key, err)
			}
		}
	}

	validateOrganizationPlan(&plan)
	return plan, nil
}

func addSplitMoves(plan *OrganizationPlan, seenSources map[string]struct{}, firstPartName, sourceFolder, destinationFolder string) error {
	entries, err := os.ReadDir(sourceFolder)
	if err != nil {
		return fmt.Errorf("read split folder %q: %w", sourceFolder, err)
	}
	partPrefix := strings.TrimSuffix(firstPartName, "00")
	if partPrefix == firstPartName {
		return fmt.Errorf("split file %q does not end with part 00", firstPartName)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || len(name) < 2 || !strings.HasPrefix(name, partPrefix) {
			continue
		}
		if _, err := strconv.Atoi(name[len(name)-2:]); err != nil {
			continue
		}
		source := filepath.Join(sourceFolder, name)
		if err := addOrganizationMove(plan, seenSources, source, filepath.Join(destinationFolder, name)); err != nil {
			return err
		}
	}
	return nil
}

func addOrganizationMove(plan *OrganizationPlan, seenSources map[string]struct{}, source, destination string) error {
	sourceKey, err := organizationPathKey(source)
	if err != nil {
		return fmt.Errorf("resolve source %q: %w", source, err)
	}
	if _, exists := seenSources[sourceKey]; exists {
		return nil
	}
	seenSources[sourceKey] = struct{}{}

	info, err := os.Lstat(source)
	if err != nil {
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, Reason: "source file is unavailable: " + err.Error()})
		return nil
	}
	if !info.Mode().IsRegular() {
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, Reason: "source is not a regular file"})
		return nil
	}
	if sameOrganizationPath(source, destination) {
		return nil
	}
	if blockedParent, parentErr := organizationDestinationParentConflict(destination); parentErr != nil {
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, OtherSource: blockedParent, Reason: "cannot use destination parent: " + parentErr.Error()})
		return nil
	} else if blockedParent != "" {
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, OtherSource: blockedParent, Reason: "destination parent is not a directory"})
		return nil
	}
	if destinationInfo, statErr := os.Lstat(destination); statErr == nil {
		if os.SameFile(info, destinationInfo) {
			return nil
		}
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, OtherSource: destination, Reason: "destination already exists"})
		return nil
	} else if !os.IsNotExist(statErr) {
		plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: source, Destination: destination, Reason: "cannot inspect destination: " + statErr.Error()})
		return nil
	}
	plan.Moves = append(plan.Moves, OrganizationMove{Source: source, Destination: destination, SourceSize: info.Size(), SourceStamp: info})
	return nil
}

func organizationDestinationParentConflict(destination string) (string, error) {
	parent := filepath.Dir(destination)
	for {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return parent, nil
			}
			return "", nil
		}
		if !os.IsNotExist(err) {
			return parent, err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", nil
		}
		parent = next
	}
}

func validateOrganizationPlan(plan *OrganizationPlan) {
	owners := make(map[string]OrganizationMove, len(plan.Moves))
	sources := make(map[string]OrganizationMove, len(plan.Moves))
	for _, move := range plan.Moves {
		key, err := organizationPathKey(move.Source)
		if err == nil {
			sources[key] = move
		}
	}
	for _, move := range plan.Moves {
		destinationKey, err := organizationPathKey(move.Destination)
		if err != nil {
			plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: move.Source, Destination: move.Destination, Reason: "cannot resolve destination: " + err.Error()})
			continue
		}
		if prior, exists := owners[destinationKey]; exists && !sameOrganizationPath(prior.Source, move.Source) {
			plan.Conflicts = append(plan.Conflicts,
				OrganizationConflict{Source: prior.Source, Destination: move.Destination, OtherSource: move.Source, Reason: "multiple files have the same destination"},
				OrganizationConflict{Source: move.Source, Destination: move.Destination, OtherSource: prior.Source, Reason: "multiple files have the same destination"},
			)
		} else {
			owners[destinationKey] = move
		}
		if occupant, exists := sources[destinationKey]; exists && !sameOrganizationPath(occupant.Source, move.Source) {
			plan.Conflicts = append(plan.Conflicts, OrganizationConflict{Source: move.Source, Destination: move.Destination, OtherSource: occupant.Source, Reason: "destination belongs to another planned source"})
		}
	}
	plan.Conflicts = uniqueOrganizationConflicts(plan.Conflicts)
}

func uniqueOrganizationConflicts(conflicts []OrganizationConflict) []OrganizationConflict {
	seen := map[string]struct{}{}
	result := make([]OrganizationConflict, 0, len(conflicts))
	for _, conflict := range conflicts {
		key := strings.Join([]string{conflict.Source, conflict.Destination, conflict.OtherSource, conflict.Reason}, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, conflict)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Destination != result[j].Destination {
			return result[i].Destination < result[j].Destination
		}
		return result[i].Source < result[j].Source
	})
	return result
}

func organizationPathKey(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		abs = strings.ToLower(abs)
	}
	return abs, nil
}

func sameOrganizationPath(left, right string) bool {
	leftKey, leftErr := organizationPathKey(left)
	rightKey, rightErr := organizationPathKey(right)
	if leftErr == nil && rightErr == nil && leftKey == rightKey {
		return true
	}
	leftInfo, leftStatErr := os.Stat(left)
	rightInfo, rightStatErr := os.Stat(right)
	return leftStatErr == nil && rightStatErr == nil && os.SameFile(leftInfo, rightInfo)
}

func organizationPathWithin(root, candidate string) (bool, error) {
	rootKey, err := organizationPathKey(root)
	if err != nil {
		return false, fmt.Errorf("resolve library folder %q: %w", root, err)
	}
	candidateKey, err := organizationPathKey(candidate)
	if err != nil {
		return false, fmt.Errorf("resolve path %q: %w", candidate, err)
	}
	if organizationPathIsWithin(rootKey, candidateKey) {
		return true, nil
	}
	rootHasSymlink, err := organizationPathHasSymlink(rootKey)
	if err != nil {
		return false, fmt.Errorf("inspect library folder path: %w", err)
	}
	candidateHasSymlink, err := organizationPathHasSymlink(candidateKey)
	if err != nil {
		return false, fmt.Errorf("inspect source folder path: %w", err)
	}
	if !rootHasSymlink && !candidateHasSymlink {
		return false, nil
	}
	resolvedRoot, err := resolveOrganizationPath(rootKey)
	if err != nil {
		return false, fmt.Errorf("resolve library folder %q: %w", root, err)
	}
	resolvedCandidate, err := resolveOrganizationPath(candidateKey)
	if err != nil {
		return false, fmt.Errorf("resolve path %q: %w", candidate, err)
	}
	resolvedRoot, err = organizationPathKey(resolvedRoot)
	if err != nil {
		return false, err
	}
	resolvedCandidate, err = organizationPathKey(resolvedCandidate)
	if err != nil {
		return false, err
	}
	return organizationPathIsWithin(resolvedRoot, resolvedCandidate), nil
}

func organizationPathIsWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		// A different drive or volume is necessarily outside the library root.
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func organizationPathHasSymlink(path string) (bool, error) {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	remainder := strings.TrimPrefix(path, current)
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}

func resolveOrganizationPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	absolute = filepath.Clean(absolute)
	hasSymlink, err := organizationPathHasSymlink(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect path for symbolic links: %w", err)
	}
	if !hasSymlink {
		return absolute, nil
	}
	missing := []string{}
	for current := absolute; ; current = filepath.Dir(current) {
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", resolveErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", resolveErr
		}
		missing = append(missing, filepath.Base(current))
	}
}

// ExecuteOrganizationPlan applies a previously validated plan and refuses to
// replace destinations that appear after preflight. Empty-folder cleanup runs
// inside baseFolder only; scanFolders nested inside it are kept.
func ExecuteOrganizationPlan(plan OrganizationPlan, localDB *db.LocalSwitchFilesDB, baseFolder string, options settings.OrganizeOptions, progress db.ProgressUpdater, scanFolders ...string) error {
	if len(plan.Conflicts) > 0 {
		return &OrganizationConflictsError{Conflicts: plan.Conflicts}
	}
	if len(plan.Moves) == 0 {
		return nil
	}
	for i, move := range plan.Moves {
		current, err := os.Lstat(move.Source)
		if err != nil {
			return fmt.Errorf("inspect source %q before move: %w", move.Source, err)
		}
		if move.SourceStamp != nil && (!os.SameFile(current, move.SourceStamp) || current.Size() != move.SourceSize || !current.ModTime().Equal(move.SourceStamp.ModTime())) {
			return fmt.Errorf("source %q changed after organization preflight", move.Source)
		}
		if err := os.MkdirAll(filepath.Dir(move.Destination), os.ModePerm); err != nil {
			return fmt.Errorf("create destination folder for %q: %w", move.Destination, err)
		}
		if progress != nil {
			progress.UpdateProgress(i+1, len(plan.Moves)+1, move.Source)
		}
		if err := moveOrganizationFile(move); err != nil {
			return fmt.Errorf("move %q to %q: %w", move.Source, move.Destination, err)
		}
		if err := updateLocalFilePaths(localDB, move.Source, move.Destination); err != nil {
			return err
		}
		zap.S().Infof("Moved file: %s -> %s", move.Source, move.Destination)
	}
	if options.DeleteEmptyFolders {
		if err := deleteEmptyOrganizationFolders(baseFolder, scanFolders); err != nil {
			return fmt.Errorf("delete empty folders: %w", err)
		}
	}
	if progress != nil {
		progress.UpdateProgress(len(plan.Moves)+1, len(plan.Moves)+1, "done")
	}
	return nil
}

func updateLocalFilePaths(localDB *db.LocalSwitchFilesDB, source, destination string) error {
	if localDB == nil {
		return nil
	}
	replacement, err := os.Stat(destination)
	if err != nil {
		return fmt.Errorf("inspect moved file at %q: %w", destination, err)
	}
	newInfo := db.ExtendedFileInfo{BaseFolder: filepath.Dir(destination), FileName: filepath.Base(destination), Size: replacement.Size(), ModTime: replacement.ModTime()}
	update := func(info *db.ExtendedFileInfo) {
		if info != nil && sameOrganizationPath(filepath.Join(info.BaseFolder, info.FileName), source) {
			*info = newInfo
		}
	}
	for _, game := range localDB.TitlesMap {
		if game == nil {
			continue
		}
		update(&game.File.ExtendedInfo)
		for version, item := range game.Updates {
			update(&item.ExtendedInfo)
			game.Updates[version] = item
		}
		for id, item := range game.Dlc {
			update(&item.ExtendedInfo)
			game.Dlc[id] = item
		}
	}
	return nil
}

// OrganizeByFolders plans every move before applying any of them.
func OrganizeByFolders(baseFolder string, localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, progress db.ProgressUpdater) error {
	settingsObj, err := settings.ReadSettings(baseFolder)
	if err != nil {
		return fmt.Errorf("read settings before organizing files: %w", err)
	}
	libraryRoot := settingsObj.Paths.LibraryFolder
	if libraryRoot == "" {
		libraryRoot = baseFolder
	}
	plan, err := BuildOrganizationPlan(libraryRoot, localDB, titlesDB, settingsObj.Organization)
	if err != nil {
		return err
	}
	return ExecuteOrganizationPlan(plan, localDB, libraryRoot, settingsObj.Organization, progress, settingsObj.Paths.ScanFolders...)
}
