package process

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
	"github.com/trembon/switch-library-manager/backend/switchfs"
)

// OrganizationPreviewEntry describes one resulting path, relative to the main
// library when possible. The preview intentionally exposes only the content
// kind and path; it does not expose the source file or any local metadata.
type OrganizationPreviewEntry struct {
	Kind string
	Path string
}

// BuildOrganizationPreview returns one example for each supported content
// kind. Entries from the scanned library are preferred, with deterministic
// examples filling any missing kinds.
func BuildOrganizationPreview(baseFolder string, options settings.OrganizeOptions, localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) ([]OrganizationPreviewEntry, error) {
	byKind := map[string]OrganizationPreviewEntry{}

	if localDB != nil {
		keys := make([]string, 0, len(localDB.TitlesMap))
		for key := range localDB.TitlesMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			game := localDB.TitlesMap[key]
			if game == nil {
				continue
			}
			var title *db.SwitchTitle
			if titlesDB != nil {
				title = titlesDB.TitlesMap[key]
			}
			entries, err := organizationPreviewForGame(baseFolder, options, game, title)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if _, exists := byKind[entry.Kind]; !exists {
					byKind[entry.Kind] = entry
				}
			}
			if len(byKind) == 3 {
				break
			}
		}
	}

	if len(byKind) < 3 {
		fallback, err := fallbackOrganizationPreview(baseFolder, options)
		if err != nil {
			return nil, err
		}
		for _, entry := range fallback {
			if _, exists := byKind[entry.Kind]; !exists {
				byKind[entry.Kind] = entry
			}
		}
	}

	result := make([]OrganizationPreviewEntry, 0, 3)
	for _, kind := range []string{"game", "update", "dlc"} {
		if entry, ok := byKind[kind]; ok {
			result = append(result, entry)
		}
	}
	return result, nil
}

func organizationPreviewForGame(baseFolder string, options settings.OrganizeOptions, game *db.SwitchGameFiles, title *db.SwitchTitle) ([]OrganizationPreviewEntry, error) {
	if !game.BaseExist && !options.ProcessWhenMissingBaseGame {
		return nil, nil
	}

	titleName := db.ResolveGameTitleName(title, game, options.AllowUnicodeFileNames)
	templateData := map[string]string{
		settings.TEMPLATE_TITLE_NAME:  titleName,
		settings.TEMPLATE_VERSION_TXT: "",
		settings.TEMPLATE_VERSION:     "0",
	}
	if title != nil {
		templateData[settings.TEMPLATE_REGION] = title.Attributes.Region
		templateData[settings.TEMPLATE_TITLE_ID] = title.Attributes.Id
	}
	if templateData[settings.TEMPLATE_TITLE_ID] == "" && game.File.Metadata != nil {
		templateData[settings.TEMPLATE_TITLE_ID] = game.File.Metadata.TitleId
	}

	gameFolder := ""
	if options.CreateFolderPerGame {
		gameFolder = getFolderName(options, templateData)
	}

	result := make([]OrganizationPreviewEntry, 0, 3)
	if game.BaseExist {
		templateData[settings.TEMPLATE_TYPE] = "BASE"
		templateData[settings.TEMPLATE_PACKAGE_CONTENTS] = packageContents(game)
		setBaseFileVersionTemplateData(templateData, game)
		setFileSizeTemplateData(templateData, game.File.ExtendedInfo.Size)
		target, included, err := organizationTargetPath(baseFolder, gameFolder, game.File.ExtendedInfo.BaseFolder, game.File.ExtendedInfo.FileName, options, "base", templateData, 0)
		if err != nil {
			return nil, fmt.Errorf("resolve base preview path: %w", err)
		}
		if included {
			result = append(result, OrganizationPreviewEntry{Kind: "game", Path: organizationPreviewPath(baseFolder, target)})
		}
	}
	templateData[settings.TEMPLATE_PACKAGE_CONTENTS] = ""

	updateVersions := make([]int, 0, len(game.Updates))
	for version := range game.Updates {
		updateVersions = append(updateVersions, version)
	}
	sort.Ints(updateVersions)
	for i := len(updateVersions) - 1; i >= 0; i-- {
		version := updateVersions[i]
		update := game.Updates[version]
		if game.BaseExist && samePhysicalFilePath(game.File.ExtendedInfo, update.ExtendedInfo) {
			continue
		}
		if update.Metadata != nil {
			templateData[settings.TEMPLATE_TITLE_ID] = update.Metadata.TitleId
		}
		templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(version)
		setFileSizeTemplateData(templateData, update.ExtendedInfo.Size)
		templateData[settings.TEMPLATE_VERSION_TXT] = ""
		if update.Metadata != nil && update.Metadata.Ncap != nil {
			templateData[settings.TEMPLATE_VERSION_TXT] = update.Metadata.Ncap.DisplayVersion
		}
		templateData[settings.TEMPLATE_TYPE] = "UPD"
		target, included, err := organizationTargetPath(baseFolder, gameFolder, update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName, options, "update", templateData, 0)
		if err != nil {
			return nil, fmt.Errorf("resolve update preview path: %w", err)
		}
		if !included {
			continue
		}
		result = append(result, OrganizationPreviewEntry{Kind: "update", Path: organizationPreviewPath(baseFolder, target)})
		break
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
		templateData[settings.TEMPLATE_TITLE_ID] = id
		templateData[settings.TEMPLATE_TYPE] = "DLC"
		templateData[settings.TEMPLATE_DLC_NAME] = getDlcName(title, dlc)
		setFileSizeTemplateData(templateData, dlc.ExtendedInfo.Size)
		target, included, err := organizationTargetPath(baseFolder, gameFolder, dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName, options, "dlc", templateData, 0)
		if err != nil {
			return nil, fmt.Errorf("resolve DLC preview path: %w", err)
		}
		if !included {
			continue
		}
		result = append(result, OrganizationPreviewEntry{Kind: "dlc", Path: organizationPreviewPath(baseFolder, target)})
		break
	}

	return result, nil
}

func fallbackOrganizationPreview(baseFolder string, options settings.OrganizeOptions) ([]OrganizationPreviewEntry, error) {
	baseID := "0100E95004039000"
	updateID := "0100E95004039800"
	dlcID := "0100E9500403A001"
	sampleFolder := baseFolder
	if sampleFolder == "" {
		sampleFolder = "."
	}
	if options.MoveScanFilesToLibrary {
		sampleFolder = filepath.Join(sampleFolder, "..", ".slm-scan-folder-preview")
	}

	game := &db.SwitchGameFiles{
		BaseExist: true,
		File: db.SwitchFileInfo{
			ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure.nsp", Size: 600_000_000},
			Metadata:     previewMetadata(baseID, 0, "1.0.0"),
		},
		Updates: map[int]db.SwitchFileInfo{
			5: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure-update.nsp", Size: 600_000_000},
				Metadata:     previewMetadata(updateID, 5, "5.0.0"),
			},
		},
		Dlc: map[string]db.SwitchFileInfo{
			dlcID: {
				ExtendedInfo: db.ExtendedFileInfo{BaseFolder: sampleFolder, FileName: "example-adventure-dlc.nsp", Size: 600_000_000},
				Metadata:     previewMetadata(dlcID, 1, "1.0.0"),
			},
		},
	}
	title := &db.SwitchTitle{
		Attributes: db.TitleAttributes{Id: baseID, Name: "Example Adventure", Region: "US"},
		Dlc: map[string]db.TitleAttributes{
			dlcID: {Id: dlcID, Name: "Example Adventure - Expansion Pack"},
		},
	}
	return organizationPreviewForGame(baseFolder, options, game, title)
}

func previewMetadata(id string, version int, displayVersion string) *switchfs.ContentMetaAttributes {
	return &switchfs.ContentMetaAttributes{
		TitleId: id,
		Version: version,
		Ncap:    &switchfs.Nacp{DisplayVersion: displayVersion},
	}
}

func organizationPreviewPath(baseFolder, target string) string {
	if baseFolder == "" || !filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	relative, err := filepath.Rel(baseFolder, target)
	if err != nil {
		return filepath.Clean(target)
	}
	return filepath.Clean(relative)
}
