package app

import (
	"path/filepath"

	"github.com/trembon/switch-library-manager/backend/db"
)

func buildLocalLibraryData(localDB *db.LocalSwitchFilesDB, switchDB *db.SwitchTitlesDB, allowUnicode bool) LocalLibraryData {
	response := LocalLibraryData{
		LibraryData: make([]LibraryTemplateData, 0),
		Issues:      make([]Pair, 0),
	}
	for key, value := range localDB.TitlesMap {
		if value.BaseExist {
			version := ""
			titleID := ""
			if value.File.Metadata != nil {
				titleID = value.File.Metadata.TitleId
			}
			var title *db.SwitchTitle
			if switchDB != nil {
				title = switchDB.TitlesMap[key]
			}
			name := db.ResolveGameTitleName(title, value, allowUnicode)
			if value.File.Metadata != nil && value.File.Metadata.Ncap != nil {
				version = value.File.Metadata.Ncap.DisplayVersion
			}

			if latest, ok := value.Updates[value.LatestUpdate]; ok && latest.Metadata != nil && latest.Metadata.Ncap != nil && latest.Metadata.Ncap.DisplayVersion != "" {
				version = latest.Metadata.Ncap.DisplayVersion
			}

			if title != nil {
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Icon:    title.Attributes.IconUrl,
					Name:    name,
					TitleId: title.Attributes.Id,
					Update:  value.LatestUpdate,
					Version: version,
					Region:  title.Attributes.Region,
					Type:    getType(value),
					Path:    filepath.Join(value.File.ExtendedInfo.BaseFolder, value.File.ExtendedInfo.FileName),
				})
			} else {
				response.LibraryData = append(response.LibraryData, LibraryTemplateData{
					Name:    name,
					Update:  value.LatestUpdate,
					Version: version,
					Type:    getType(value),
					TitleId: titleID,
					Path:    filepath.Join(value.File.ExtendedInfo.BaseFolder, value.File.ExtendedInfo.FileName),
				})
			}
		} else {
			for _, update := range value.Updates {
				response.Issues = append(response.Issues, Pair{
					Key:   filepath.Join(update.ExtendedInfo.BaseFolder, update.ExtendedInfo.FileName),
					Value: "base file is missing",
				})
			}
			for _, dlc := range value.Dlc {
				response.Issues = append(response.Issues, Pair{
					Key:   filepath.Join(dlc.ExtendedInfo.BaseFolder, dlc.ExtendedInfo.FileName),
					Value: "base file is missing",
				})
			}
		}
	}
	for key, skipped := range localDB.Skipped {
		response.Issues = append(response.Issues, Pair{
			Key:   filepath.Join(key.BaseFolder, key.FileName),
			Value: skipped.ReasonText,
		})
	}
	response.NumFiles = localDB.NumFiles
	return response
}

func getType(gameFile *db.SwitchGameFiles) string {
	if gameFile.IsSplit {
		return "split"
	}
	if gameFile.MultiContent {
		return "multi-content"
	}
	ext := filepath.Ext(gameFile.File.ExtendedInfo.FileName)
	if len(ext) > 1 {
		return ext[1:]
	}
	return ""
}
