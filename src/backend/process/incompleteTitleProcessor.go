package process

import (
	"sort"
	"strconv"
	"strings"

	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/switchfs"
	"go.uber.org/zap"
)

type MissingDLC struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	ReleaseDate string `json:"release_date"`
}

type IncompleteTitle struct {
	Attributes          db.TitleAttributes
	Meta                *switchfs.ContentMetaAttributes
	LocalUpdate         int          `json:"local_update"`
	LocalDisplayVersion string       `json:"local_display_version"`
	LatestUpdate        int          `json:"latest_update"`
	LatestUpdateDate    string       `json:"latest_update_date"`
	MissingDLC          []MissingDLC `json:"missing_dlc"`
}

func ScanForMissingUpdates(localDB map[string]*db.SwitchGameFiles,
	switchDB map[string]*db.SwitchTitle,
	ignoreTitleIds map[string]struct{},
	ignoreDLCupdates bool) map[string]IncompleteTitle {

	result := map[string]IncompleteTitle{}

	//iterate over local files, and compare to remote versions
	for idPrefix, switchFile := range localDB {

		if !switchFile.BaseExist {
			zap.S().Infof("missing base for game %v", idPrefix)
			continue
		}

		if _, ok := switchDB[idPrefix]; !ok {
			continue
		}

		if _, ok := ignoreTitleIds[strings.ToLower(switchDB[idPrefix].Attributes.Id)]; ok {
			continue
		}

		switchTitle := IncompleteTitle{Attributes: switchDB[idPrefix].Attributes, Meta: switchFile.File.Metadata}

		//sort the available local versions
		localVersions := make([]int, len(switchFile.Updates))
		i := 0
		for k := range switchFile.Updates {
			localVersions[i] = k
			i++
		}
		sort.Ints(localVersions)

		//sort the available remote versions
		remoteVersions := make([]int, len(switchDB[idPrefix].Updates))
		i = 0
		for k := range switchDB[idPrefix].Updates {
			remoteVersions[i] = k
			i++
		}
		sort.Ints(remoteVersions)
		switchTitle.LocalUpdate = 0
		switchTitle.LatestUpdate = 0
		if len(localVersions) != 0 {
			switchTitle.LocalUpdate = localVersions[len(localVersions)-1]
			localUpdate := switchFile.Updates[switchTitle.LocalUpdate]
			switchTitle.LocalDisplayVersion = displayVersion(localUpdate.Metadata)
		} else {
			switchTitle.LocalDisplayVersion = displayVersion(switchFile.File.Metadata)
		}

		//process updates
		if len(remoteVersions) != 0 {
			switchTitle.LatestUpdate = remoteVersions[len(remoteVersions)-1]
			switchTitle.LatestUpdateDate = switchDB[idPrefix].Updates[remoteVersions[len(remoteVersions)-1]]
			if switchTitle.LocalUpdate < switchTitle.LatestUpdate {
				result[switchDB[idPrefix].Attributes.Id] = switchTitle
			}
		}

		if len(switchDB[idPrefix].Dlc) == 0 {
			continue
		}

		//process dlc
		if !ignoreDLCupdates {
			for k, availableDlc := range switchDB[idPrefix].Dlc {

				if localDlc, ok := switchFile.Dlc[k]; ok {
					latestDlcVersion, err := availableDlc.Version.Int64()
					if err != nil {
						continue
					}

					if localDlc.Metadata == nil {
						continue
					}

					if _, ok := ignoreTitleIds[strings.ToLower(localDlc.Metadata.TitleId)]; ok {
						continue
					}

					if localDlc.Metadata.Version < int(latestDlcVersion) {
						updateDate := "-"
						if availableDlc.ReleaseDate != 0 {
							updateDate = strconv.Itoa(availableDlc.ReleaseDate)
							if len(updateDate) > 7 {
								updateDate = updateDate[0:4] + "-" + updateDate[4:6] + "-" + updateDate[6:]
							}
						}

						result[availableDlc.Id] = IncompleteTitle{
							Attributes:          availableDlc,
							LatestUpdate:        int(latestDlcVersion),
							LocalUpdate:         localDlc.Metadata.Version,
							LocalDisplayVersion: displayVersion(localDlc.Metadata),
							LatestUpdateDate:    updateDate,
							Meta:                localDlc.Metadata}
					}
				}
			}
		}

	}
	return result
}

func displayVersion(metadata *switchfs.ContentMetaAttributes) string {
	if metadata == nil || metadata.Ncap == nil {
		return ""
	}
	return metadata.Ncap.DisplayVersion
}

func ScanForMissingDLC(localDB map[string]*db.SwitchGameFiles,
	switchDB map[string]*db.SwitchTitle, ignoreTitleIds map[string]struct{}) map[string]IncompleteTitle {
	result := map[string]IncompleteTitle{}

	//iterate over local files, and compare to remote versions
	for idPrefix, switchFile := range localDB {

		if !switchFile.BaseExist {
			continue
		}

		if _, ok := switchDB[idPrefix]; !ok {
			continue
		}
		switchTitle := IncompleteTitle{Attributes: switchDB[idPrefix].Attributes}

		//process dlc
		if len(switchDB[idPrefix].Dlc) != 0 {
			for k, v := range switchDB[idPrefix].Dlc {
				if _, ok := ignoreTitleIds[k]; ok {
					continue
				}

				if _, ok := switchFile.Dlc[k]; !ok {
					releaseDate := ""
					if v.ReleaseDate != 0 {
						releaseDate = v.ParsedReleaseDate
					}
					switchTitle.MissingDLC = append(switchTitle.MissingDLC, MissingDLC{Id: v.Id, Name: v.Name, ReleaseDate: releaseDate})
				}
			}
			if len(switchTitle.MissingDLC) != 0 {
				result[switchDB[idPrefix].Attributes.Id] = switchTitle
			}
		}
	}
	return result
}

func ScanForBrokenFiles(localDB map[string]*db.SwitchGameFiles) []db.SwitchFileInfo {
	var result []db.SwitchFileInfo

	//iterate over local files, and compare to remote versions
	for _, switchFile := range localDB {

		if !switchFile.BaseExist {
			for _, f := range switchFile.Dlc {
				result = append(result, f)
			}
			for _, f := range switchFile.Updates {
				result = append(result, f)
			}
		}
	}
	return result
}
