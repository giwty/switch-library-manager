package process

import (
	"fmt"
	"github.com/giwty/switch-backup-manager/db"
	"sort"
	"strconv"
)

type incompleteTitle struct {
	Attributes       db.TitleAttributes
	LocalUpdate      int
	LatestUpdate     int
	LatestUpdateDate string
	MissingDLC       []string
}

func ScanForMissingUpdates(localDB map[string]*db.SwitchFile, switchDB map[string]*db.SwitchTitle) map[string]incompleteTitle {
	result := map[string]incompleteTitle{}

	//iterate over local files, and compare to remote versions
	for idPrefix, switchFile := range localDB {

		if switchFile.BaseExist == false {
			continue
		}
		switchTitle := incompleteTitle{Attributes: switchDB[idPrefix].Attributes}
		//sort the available local versions
		localVersions := make([]int, len(switchFile.Updates))
		i := 0
		for k, _ := range switchFile.Updates {
			localVersions[i] = k
			i++
		}
		sort.Ints(localVersions)

		//sort the available remote versions
		remoteVersions := make([]int, len(switchDB[idPrefix].Updates))
		i = 0
		for k, _ := range switchDB[idPrefix].Updates {
			remoteVersions[i] = k
			i++
		}
		sort.Ints(remoteVersions)
		switchTitle.LocalUpdate = 0
		switchTitle.LatestUpdate = 0
		if len(localVersions) != 0 {
			switchTitle.LocalUpdate = localVersions[len(localVersions)-1]
		}

		//process updates
		if len(remoteVersions) != 0 {
			switchTitle.LatestUpdate = remoteVersions[len(remoteVersions)-1]
			switchTitle.LatestUpdateDate = switchDB[idPrefix].Updates[remoteVersions[len(remoteVersions)-1]]
			if switchTitle.LocalUpdate != switchTitle.LatestUpdate {
				result[switchDB[idPrefix].Attributes.Id] = switchTitle
			}
		}

		if len(switchDB[idPrefix].Dlc) == 0 {
			continue
		}

		//process dlc
		for k, availableDlc := range switchDB[idPrefix].Dlc {
			if localDlc, ok := switchFile.Dlc[k]; ok {
				latestDlcVersion, err := availableDlc.Version.Int64()
				if err != nil {
					continue
				}

				if localDlc.Metadata == nil {
					continue
				}
				if localDlc.Metadata.Version != int(latestDlcVersion) {
					result[availableDlc.Id] = incompleteTitle{Attributes: availableDlc, LatestUpdate: int(latestDlcVersion), LocalUpdate: localDlc.Metadata.Version, LatestUpdateDate: strconv.Itoa(availableDlc.ReleaseDate)}
				}
			}
		}

	}
	return result
}

func ScanForMissingDLC(localDB map[string]*db.SwitchFile, switchDB map[string]*db.SwitchTitle) map[string]incompleteTitle {
	result := map[string]incompleteTitle{}

	//iterate over local files, and compare to remote versions
	for idPrefix, switchFile := range localDB {

		if switchFile.BaseExist == false {
			continue
		}
		switchTitle := incompleteTitle{Attributes: switchDB[idPrefix].Attributes}

		//process dlc
		if len(switchDB[idPrefix].Dlc) != 0 {
			for k, v := range switchDB[idPrefix].Dlc {
				if _, ok := switchFile.Dlc[k]; !ok {
					switchTitle.MissingDLC = append(switchTitle.MissingDLC, fmt.Sprintf("%v [%v]", v.Id, v.Name))
				}
			}
			if len(switchTitle.MissingDLC) != 0 {
				result[switchDB[idPrefix].Attributes.Id] = switchTitle
			}
		}
	}
	return result
}

func ScanForBrokenFiles(localDB map[string]*db.SwitchFile) []db.ExtendedFileInfo {
	var result []db.ExtendedFileInfo

	//iterate over local files, and compare to remote versions
	for _, switchFile := range localDB {

		if switchFile.BaseExist == false {
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
