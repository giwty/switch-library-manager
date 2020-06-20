package process

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"switch/nsp-update/db"
)

var (
	folderIllegalCharsRegex = regexp.MustCompile(`/[/\\?%*:|"<>]/g`)
)

func DeleteOldUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {
	for _, v := range localDB.TitlesMap {

		if len(v.Updates) > 1 {
			//sort the available local versions
			localVersions := make([]int, len(v.Updates))
			i := 0
			for k, _ := range v.Updates {
				localVersions[i] = k
				i++
			}
			sort.Ints(localVersions)

			for i := 0; i < len(localVersions)-1; i++ {
				fileToRemove := filepath.Join(v.Updates[localVersions[i]].BaseFolder, v.Updates[localVersions[i]].Info.Name())
				fmt.Printf("--> [Delete] Old update file: %v [latest update:%v]\n", fileToRemove, localVersions[len(localVersions)-1])
				err := os.Remove(fileToRemove)
				if err != nil {
					fmt.Printf("Failed to delete file  %v  [%v]\n", fileToRemove, err)
				}
			}
			v.Updates = map[int]db.ExtendedFileInfo{localVersions[len(localVersions)-1]: v.Updates[localVersions[len(localVersions)-1]]}
		}

	}
}

func OrganizeByFolders(baseFolder string, localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {

	for k, v := range localDB.TitlesMap {
		if v.BaseExist == false {
			continue
		}

		destinationFolderName := titlesDB.TitlesMap[k].Attributes.Name + " [" + titlesDB.TitlesMap[k].Attributes.Id + "]"
		destinationFolderName = folderIllegalCharsRegex.ReplaceAllString(destinationFolderName, "-")
		destinationPath := filepath.Join(baseFolder, destinationFolderName)
		if _, err := os.Stat(destinationPath); os.IsNotExist(err) {
			err = os.Mkdir(destinationPath, os.ModePerm)
			if err != nil {
				fmt.Printf("Failed to create folder %v - %v\n", destinationFolderName, err)
				continue
			}
		}

		//move base title
		currLocation := filepath.Join(v.File.BaseFolder, v.File.Info.Name())
		newLocation := filepath.Join(destinationPath, v.File.Info.Name())
		if currLocation != newLocation {
			err := os.Rename(currLocation, newLocation)
			if err != nil {
				fmt.Printf("Failed to move file  %v to %v [%v]\n", currLocation, newLocation, err)
				continue
			}
		}

		//move updates
		for _, update := range v.Updates {
			currLocation = filepath.Join(update.BaseFolder, update.Info.Name())
			newLocation = filepath.Join(destinationPath, update.Info.Name())
			if currLocation == newLocation {
				continue
			}
			err := os.Rename(currLocation, newLocation)
			if err != nil {
				fmt.Printf("Failed to move file  %v to %v [%v]\n", currLocation, newLocation, err)
				continue
			}
		}

		//move DLC
		for _, dlc := range v.Dlc {
			currLocation = filepath.Join(dlc.BaseFolder, dlc.Info.Name())
			newLocation = filepath.Join(destinationPath, dlc.Info.Name())
			if currLocation == newLocation {
				continue
			}
			err := os.Rename(currLocation, newLocation)
			if err != nil {
				fmt.Printf("Failed to move file  %v to %v [%v]\n", currLocation, newLocation, err)
				continue
			}
		}
	}
}
