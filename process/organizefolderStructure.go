package process

import (
	"fmt"
	"github.com/giwty/switch-backup-manager/db"
	"github.com/giwty/switch-backup-manager/settings"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	folderIllegalCharsRegex = regexp.MustCompile(`[/\\?%*:|"<>]`)
)

func DeleteOldUpdates(localDB *db.LocalSwitchFilesDB) {
	for _, v := range localDB.TitlesMap {

		if len(v.Updates) > 1 {
			//sort the available local versions
			localVersions := make([]int, len(v.Updates))
			i := 0
			for k := range v.Updates {
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

func OrganizeByFolders(baseFolder string, localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, updateProgress db.ProgressUpdater) {

	options := settings.ReadSettings().OrganizeOptions
	i := 0
	for k, v := range localDB.TitlesMap {
		i++
		if v.BaseExist == false {
			continue
		}
		if updateProgress != nil {
			updateProgress.UpdateProgress(i, len(localDB.TitlesMap), v.File.Info.Name())
		}

		titleName := getTitleName(titlesDB.TitlesMap[k], v)

		templateData := map[string]string{}

		templateData[settings.TEMPLATE_TITLE_ID] = v.File.Metadata.TitleId
		//templateData[settings.TEMPLATE_TYPE] = "BASE"
		templateData[settings.TEMPLATE_TITLE_NAME] = titleName
		templateData[settings.TEMPLATE_VERSION] = "0"

		var destinationPath = v.File.BaseFolder

		//create folder if needed
		if options.CreateFolderPerGame {
			folderToCreate := getFolderName(options, templateData)
			destinationPath = filepath.Join(baseFolder, folderToCreate)
			if _, err := os.Stat(destinationPath); os.IsNotExist(err) {
				err = os.Mkdir(destinationPath, os.ModePerm)
				if err != nil {
					fmt.Printf("Failed to create folder %v - %v\n", folderToCreate, err)
					continue
				}
			}
		}

		//process base title
		from := filepath.Join(v.File.BaseFolder, v.File.Info.Name())
		to := filepath.Join(destinationPath, getFileName(options, v.File.Info.Name(), templateData))
		err := moveFile(from, to)
		if err != nil {
			fmt.Printf("Failed to move file [%v]\n", err)
			continue
		}

		//process updates
		for update, updateInfo := range v.Updates {
			if updateInfo.Metadata != nil {
				templateData[settings.TEMPLATE_TITLE_ID] = updateInfo.Metadata.TitleId
			}
			templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(update)
			templateData[settings.TEMPLATE_TYPE] = "UPD"
			from = filepath.Join(updateInfo.BaseFolder, updateInfo.Info.Name())
			if options.CreateFolderPerGame {
				to = filepath.Join(destinationPath, getFileName(options, updateInfo.Info.Name(), templateData))
			} else {
				to = filepath.Join(updateInfo.BaseFolder, getFileName(options, updateInfo.Info.Name(), templateData))
			}
			err := moveFile(from, to)
			if err != nil {
				fmt.Printf("Failed to move file [%v]\n", err)
				continue
			}
		}

		//process DLC
		for id, dlc := range v.Dlc {
			if dlc.Metadata != nil {
				templateData[settings.TEMPLATE_VERSION] = strconv.Itoa(dlc.Metadata.Version)
			}
			templateData[settings.TEMPLATE_TYPE] = "DLC"
			templateData[settings.TEMPLATE_TITLE_ID] = id
			templateData[settings.TEMPLATE_DLC_NAME] = getDlcName(titlesDB.TitlesMap[k], dlc)
			from = filepath.Join(dlc.BaseFolder, dlc.Info.Name())
			if options.CreateFolderPerGame {
				to = filepath.Join(destinationPath, getFileName(options, dlc.Info.Name(), templateData))
			} else {
				to = filepath.Join(dlc.BaseFolder, getFileName(options, dlc.Info.Name(), templateData))
			}
			err = moveFile(from, to)
			if err != nil {
				fmt.Printf("Failed to move file [%v]\n", err)
				continue
			}
		}
	}

	if options.DeleteEmptyFolders {
		err := deleteEmptyFolders(baseFolder)
		if err != nil {
			fmt.Printf("Failed to delete empty folders [%v]\n", err)
		}
	}
}

func getDlcName(switchTitle *db.SwitchTitle, file db.ExtendedFileInfo) string {
	if switchTitle == nil {
		return ""
	}
	if dlcAttributes, ok := switchTitle.Dlc[file.Metadata.TitleId]; ok {
		name := dlcAttributes.Name
		name = strings.ReplaceAll(name, "\n", "")
		return folderIllegalCharsRegex.ReplaceAllString(name, "-")
	}
	return ""
}

func getTitleName(switchTitle *db.SwitchTitle, v *db.SwitchFile) string {
	if switchTitle != nil && switchTitle.Attributes.Name != "" {
		name := switchTitle.Attributes.Name
		return folderIllegalCharsRegex.ReplaceAllString(name, "-")
	} else {
		//for non eshop games (cartridge only), grab the name from the file
		return db.ParseTitleNameFromFileName(v.File.Info.Name())
	}
}

func getFolderName(options settings.OrganizeOptions, templateData map[string]string) string {

	return applyTemplate(templateData, options.FolderNameTemplate)
}

func getFileName(options settings.OrganizeOptions, originalName string, templateData map[string]string) string {
	if !options.RenameFiles {
		return originalName
	}
	ext := path.Ext(originalName)
	result := applyTemplate(templateData, options.FileNameTemplate)
	return result + ext
}

func moveFile(from string, to string) error {
	if from == to {
		return nil
	}
	err := os.Rename(from, to)
	return err
}

func applyTemplate(templateData map[string]string, template string) string {
	result := strings.Replace(template, "{"+settings.TEMPLATE_TITLE_NAME+"}", templateData[settings.TEMPLATE_TITLE_NAME], 1)
	result = strings.Replace(result, "{"+settings.TEMPLATE_TITLE_ID+"}", strings.ToUpper(templateData[settings.TEMPLATE_TITLE_ID]), 1)
	result = strings.Replace(result, "{"+settings.TEMPLATE_VERSION+"}", templateData[settings.TEMPLATE_VERSION], 1)
	result = strings.Replace(result, "{"+settings.TEMPLATE_TYPE+"}", templateData[settings.TEMPLATE_TYPE], 1)
	result = strings.Replace(result, "{"+settings.TEMPLATE_DLC_NAME+"}", templateData[settings.TEMPLATE_DLC_NAME], 1)
	result = strings.ReplaceAll(result, "[]", "")
	result = strings.ReplaceAll(result, "()", "")
	result = strings.ReplaceAll(result, "<>", "")
	result = strings.ReplaceAll(result, "  ", " ")
	result = strings.TrimSpace(result)
	if strings.HasSuffix(result, ".") {
		result = result[:len(result)-1]
	}
	return result
}

func deleteEmptyFolders(path string) error {
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			deleteEmptyFolder(path)
		}

		return nil
	})
	return err
}

func deleteEmptyFolder(path string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	if len(files) != 0 {
		return nil
	}

	fmt.Printf("\nDeleting empty folder [%v]", path)
	_ = os.Remove(path)

	return nil
}
