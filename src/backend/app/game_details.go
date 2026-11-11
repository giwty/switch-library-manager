package app

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/settings"
)

type GameDetails struct {
	Attributes        db.TitleAttributes `json:"attributes"`
	TitleID           string             `json:"titleId"`
	Name              string             `json:"name"`
	AutomaticName     string             `json:"automatic_name"`
	CustomName        string             `json:"custom_name"`
	ReleaseDateText   string             `json:"release_date_text"`
	RemoteAvailable   bool               `json:"remote_available"`
	MetadataAvailable bool               `json:"metadata_available"`
	BaseCollected     bool               `json:"base_collected"`
	BasePath          string             `json:"base_path"`
	Updates           []GameUpdateDetail `json:"updates"`
	LocalOnlyUpdates  []GameUpdateDetail `json:"local_only_updates"`
	DLC               []GameDLCDetail    `json:"dlc"`
	LocalOnlyDLC      []GameDLCDetail    `json:"local_only_dlc"`
}

type GameUpdateDetail struct {
	Version int    `json:"version"`
	Date    string `json:"date"`
	Status  string `json:"status"`
	Path    string `json:"path"`
}

type GameDLCDetail struct {
	TitleID          string             `json:"titleId"`
	Name             string             `json:"name"`
	Attributes       db.TitleAttributes `json:"attributes"`
	Status           string             `json:"status"`
	UpdateStatus     string             `json:"update_status"`
	LocalVersion     int                `json:"local_version"`
	HasLocalVersion  bool               `json:"has_local_version"`
	AvailableVersion int                `json:"available_version"`
	HasAvailable     bool               `json:"has_available_version"`
	Path             string             `json:"path"`
}

// GetGameDetails returns the catalog and local collection state for the game
// that owns titleID. The supplied ID may identify the base game, an update, or
// a DLC item.
func (a *App) GetGameDetails(titleID string) (GameDetails, error) {
	a.state.mu.Lock()
	defer a.state.mu.Unlock()

	if a.state.switchDB == nil || a.state.localDB == nil {
		return GameDetails{}, errors.New("local and title databases must be loaded")
	}

	settingsObj, err := settings.ReadSettings(a.baseFolder)
	if err != nil {
		return GameDetails{}, fmt.Errorf("load missing-content settings: %w", err)
	}

	return buildGameDetails(titleID, a.state.localDB, a.state.switchDB, settingsObj.MissingContent, settingsObj.Organization.AllowUnicodeFileNames, settingsObj.TitleNameOverrides)
}

func buildGameDetails(
	titleID string,
	localDB *db.LocalSwitchFilesDB,
	switchDB *db.SwitchTitlesDB,
	missingSettings settings.MissingContentSettings,
	allowUnicode bool,
	overrides ...map[string]string,
) (GameDetails, error) {
	prefix, err := db.TitleIDPrefix(titleID)
	if err != nil {
		return GameDetails{}, fmt.Errorf("resolve game title ID: %w", err)
	}
	if localDB == nil || switchDB == nil {
		return GameDetails{}, errors.New("local and title databases must be loaded")
	}

	local := localDB.TitlesMap[prefix]
	remote := switchDB.TitlesMap[prefix]
	if local == nil && remote == nil {
		return GameDetails{}, fmt.Errorf("game title ID %q was not found", strings.ToLower(titleID))
	}

	baseID := prefix + "000"
	details := GameDetails{
		TitleID:          baseID,
		Updates:          []GameUpdateDetail{},
		LocalOnlyUpdates: []GameUpdateDetail{},
		DLC:              []GameDLCDetail{},
		LocalOnlyDLC:     []GameDLCDetail{},
	}
	if remote != nil {
		details.RemoteAvailable = true
		details.Attributes = safeTitleAttributes(remote.Attributes)
		details.MetadataAvailable = hasCatalogMetadata(remote.Attributes)
	}
	if details.Attributes.Id == "" {
		details.Attributes.Id = baseID
	}
	if details.Attributes.ReleaseDate != 0 {
		details.ReleaseDateText = details.Attributes.ParsedReleaseDate
	}
	if local != nil {
		details.BaseCollected = local.BaseExist
		if local.BaseExist {
			details.BasePath = switchFilePath(local.File)
		}
	}
	details.AutomaticName = db.ResolveGameTitleName(remote, local, allowUnicode)
	details.CustomName = db.GameName(baseID, "", overrides...)
	details.Attributes.Name = db.GameName(baseID, details.AutomaticName, overrides...)
	details.Name = details.Attributes.Name

	ignoredUpdates := makeIgnoreIDs(missingSettings.IgnoreUpdateIDs)
	_, gameUpdatesIgnored := ignoredUpdates[baseID]
	ignoredDLC := makeIgnoreIDs(missingSettings.IgnoreDLCTitleIDs)
	latestLocalUpdateVersion, hasLocalUpdate := 0, false
	if local != nil {
		for version := range local.Updates {
			if !hasLocalUpdate || version > latestLocalUpdateVersion {
				latestLocalUpdateVersion = version
				hasLocalUpdate = true
			}
		}
	}

	if remote != nil {
		versions := make([]int, 0, len(remote.Updates))
		for version := range remote.Updates {
			versions = append(versions, version)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(versions)))
		for _, version := range versions {
			update := GameUpdateDetail{Version: version, Date: remote.Updates[version], Status: "missing"}
			if local != nil {
				if localUpdate, ok := local.Updates[version]; ok {
					update.Status = "collected"
					update.Path = switchFilePath(localUpdate)
				} else if hasLocalUpdate && latestLocalUpdateVersion >= version {
					update.Status = "collected"
				}
			}
			if update.Status == "missing" {
				if _, ignored := ignoredUpdates[baseID]; ignored {
					update.Status = "ignored"
				}
			}
			details.Updates = append(details.Updates, update)
		}

		dlcIDs := make([]string, 0, len(remote.Dlc))
		for id := range remote.Dlc {
			dlcIDs = append(dlcIDs, strings.ToLower(id))
		}
		sort.Slice(dlcIDs, func(i, j int) bool {
			left := remote.Dlc[dlcIDs[i]]
			right := remote.Dlc[dlcIDs[j]]
			if left.ReleaseDate != right.ReleaseDate {
				return left.ReleaseDate > right.ReleaseDate
			}
			if !strings.EqualFold(left.Name, right.Name) {
				return strings.ToLower(left.Name) < strings.ToLower(right.Name)
			}
			return dlcIDs[i] < dlcIDs[j]
		})
		for _, id := range dlcIDs {
			attributes := safeTitleAttributes(remote.Dlc[id])
			if attributes.Id == "" {
				attributes.Id = id
			}
			dlcDetail := GameDLCDetail{
				TitleID:    id,
				Name:       attributes.Name,
				Attributes: attributes,
				Status:     "missing",
			}
			localDLC, collected := findLocalDLC(local, id)
			if collected {
				dlcDetail.Status = "collected"
				dlcDetail.Path = switchFilePath(localDLC)
				if localDLC.Metadata != nil {
					dlcDetail.LocalVersion = localDLC.Metadata.Version
					dlcDetail.HasLocalVersion = true
				}
			} else if _, ignored := ignoredDLC[id]; ignored {
				dlcDetail.Status = "ignored"
			}

			if attributes.Version != "" {
				if version, parseErr := attributes.Version.Int64(); parseErr == nil && version >= 0 && version <= int64(^uint(0)>>1) {
					dlcDetail.AvailableVersion = int(version)
					dlcDetail.HasAvailable = true
				}
			}
			if !collected {
				dlcDetail.UpdateStatus = "not_collected"
			} else if !dlcDetail.HasAvailable || !dlcDetail.HasLocalVersion {
				dlcDetail.UpdateStatus = "unknown"
			} else if dlcDetail.LocalVersion >= dlcDetail.AvailableVersion {
				dlcDetail.UpdateStatus = "up_to_date"
			} else if missingSettings.IgnoreDLCUpdates || gameUpdatesIgnored {
				dlcDetail.UpdateStatus = "ignored"
			} else if _, ignored := ignoredUpdates[id]; ignored {
				dlcDetail.UpdateStatus = "ignored"
			} else {
				dlcDetail.UpdateStatus = "out_of_date"
			}
			if dlcDetail.Name == "" && collected {
				dlcDetail.Name = localFileName(localDLC)
			}
			details.DLC = append(details.DLC, dlcDetail)
		}
	}

	if local != nil {
		localVersions := make([]int, 0, len(local.Updates))
		for version := range local.Updates {
			if remote == nil {
				localVersions = append(localVersions, version)
				continue
			}
			if _, available := remote.Updates[version]; !available {
				localVersions = append(localVersions, version)
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(localVersions)))
		for _, version := range localVersions {
			details.LocalOnlyUpdates = append(details.LocalOnlyUpdates, GameUpdateDetail{
				Version: version,
				Status:  "collected",
				Path:    switchFilePath(local.Updates[version]),
			})
		}

		localDLCIDs := make([]string, 0, len(local.Dlc))
		for id := range local.Dlc {
			if remote == nil {
				localDLCIDs = append(localDLCIDs, strings.ToLower(id))
				continue
			}
			if _, available := remote.Dlc[strings.ToLower(id)]; !available {
				localDLCIDs = append(localDLCIDs, strings.ToLower(id))
			}
		}
		sort.Strings(localDLCIDs)
		for _, id := range localDLCIDs {
			localDLC, _ := findLocalDLC(local, id)
			name := localFileName(localDLC)
			attributes := db.TitleAttributes{Id: id, Name: name}
			detail := GameDLCDetail{
				TitleID:      id,
				Name:         name,
				Attributes:   attributes,
				Status:       "collected",
				UpdateStatus: "unknown",
				Path:         switchFilePath(localDLC),
			}
			if localDLC.Metadata != nil {
				detail.LocalVersion = localDLC.Metadata.Version
				detail.HasLocalVersion = true
			}
			details.LocalOnlyDLC = append(details.LocalOnlyDLC, detail)
		}
	}

	return details, nil
}

func hasCatalogMetadata(attributes db.TitleAttributes) bool {
	return attributes.Name != "" || attributes.Publisher != "" || attributes.Region != "" || attributes.ReleaseDate != 0 || attributes.Description != "" || attributes.IconUrl != "" || attributes.BannerUrl != "" || len(attributes.Screenshots) != 0
}

func safeTitleAttributes(attributes db.TitleAttributes) db.TitleAttributes {
	attributes.IconUrl = safeHTTPURL(attributes.IconUrl)
	attributes.BannerUrl = safeHTTPURL(attributes.BannerUrl)
	attributes.Screenshots = append([]string(nil), attributes.Screenshots...)
	for index := range attributes.Screenshots {
		attributes.Screenshots[index] = safeHTTPURL(attributes.Screenshots[index])
	}
	return attributes
}

func safeHTTPURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) || parsed.User != nil {
		return ""
	}
	return parsed.String()
}

func switchFileTitleName(file db.SwitchFileInfo) string {
	if file.Metadata == nil || file.Metadata.Ncap == nil {
		return ""
	}
	if entry, ok := file.Metadata.Ncap.TitleName["AmericanEnglish"]; ok {
		return entry.Title
	}
	return ""
}

func localFileName(file db.SwitchFileInfo) string {
	name := switchFileTitleName(file)
	if name != "" {
		return name
	}
	return db.ParseTitleNameFromFileName(file.ExtendedInfo.FileName)
}

func switchFilePath(file db.SwitchFileInfo) string {
	if file.ExtendedInfo.BaseFolder == "" {
		return file.ExtendedInfo.FileName
	}
	return filepath.Join(file.ExtendedInfo.BaseFolder, file.ExtendedInfo.FileName)
}

func findLocalDLC(local *db.SwitchGameFiles, titleID string) (db.SwitchFileInfo, bool) {
	if local == nil {
		return db.SwitchFileInfo{}, false
	}
	if file, ok := local.Dlc[strings.ToLower(titleID)]; ok {
		return file, true
	}
	for id, file := range local.Dlc {
		if strings.EqualFold(id, titleID) {
			return file, true
		}
	}
	return db.SwitchFileInfo{}, false
}
