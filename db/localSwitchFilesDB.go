package db

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"switch-backup-manager/settings"
	"switch-backup-manager/switchfs"
)

var (
	versionRegex = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})\]`)
	titleIdRegex = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z,0-9]{16})\]`)
)

type ExtendedFileInfo struct {
	Info       os.FileInfo
	BaseFolder string
	Metadata   *switchfs.ContentMetaAttributes
}

type SwitchFile struct {
	File      ExtendedFileInfo
	BaseExist bool
	Updates   map[int]ExtendedFileInfo
	Dlc       map[string]ExtendedFileInfo
}

type LocalSwitchFilesDB struct {
	TitlesMap map[string]*SwitchFile
	Skipped   map[os.FileInfo]string
}

func CreateLocalSwitchFilesDB(files []os.FileInfo, parentFolder string, recursive bool) (*LocalSwitchFilesDB, error) {
	titles := map[string]*SwitchFile{}
	skipped := map[os.FileInfo]string{}

	scanLocalFiles(parentFolder, files, recursive, titles, skipped)

	return &LocalSwitchFilesDB{TitlesMap: titles, Skipped: skipped}, nil
}

func scanLocalFiles(parentFolder string, files []os.FileInfo,
	recurse bool, titles map[string]*SwitchFile,
	skipped map[os.FileInfo]string) {

	for _, file := range files {
		//skip mac hidden files
		if file.Name()[0:1] == "." {
			continue
		}

		//scan sub-folders if flag is present
		filePath := filepath.Join(parentFolder, file.Name())
		if file.IsDir() {
			if !recurse {
				continue
			}
			folder := filePath
			innerFiles, err := ioutil.ReadDir(folder)
			if err != nil {
				fmt.Printf("\nfailed scanning NSP folder [%v]", err)
				continue
			}
			scanLocalFiles(folder, innerFiles, recurse, titles, skipped)
		}

		//only handle NSZ and NSP files
		if !strings.HasSuffix(file.Name(), "nsp") && !strings.HasSuffix(file.Name(), "nsz") {
			skipped[file] = "non supported File"
			continue
		}

		metadata, err := GetGameMetadata(file, filePath)

		if err != nil {
			skipped[file] = "unable to determine titileId / version"
			continue
		}

		idPrefix := metadata.TitleId[0 : len(metadata.TitleId)-4]
		switchTitle := &SwitchFile{Updates: map[int]ExtendedFileInfo{}, Dlc: map[string]ExtendedFileInfo{}, BaseExist: false}
		if t, ok := titles[idPrefix]; ok {
			switchTitle = t
		}
		titles[idPrefix] = switchTitle

		//process Updates
		if strings.HasSuffix(metadata.TitleId, "800") {
			switchTitle.Updates[metadata.Version] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
			continue
		}

		//process base
		if strings.HasSuffix(metadata.TitleId, "000") {
			switchTitle.File = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
			switchTitle.BaseExist = true
			continue
		}

		//not an update, and not main TitleAttributes, so treat it as a DLC
		switchTitle.Dlc[metadata.TitleId] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
	}

}

func GetGameMetadata(file os.FileInfo, filePath string) (*switchfs.ContentMetaAttributes, error) {
	var metadata *switchfs.ContentMetaAttributes = nil
	keys, _ := settings.SwitchKeys()
	var err error

	//currently only NSP files are supported
	if keys != nil && strings.HasSuffix(file.Name(), "nsp") {
		metadata, err = switchfs.ReadNspMetadata(filePath)
		if err != nil {
			fmt.Printf("\n[file:%v] failed to read NSP [reason: %v]\n", file.Name(), err)
		}
	}

	if metadata != nil {
		return metadata, nil
	}

	//fallback to parse data from filename

	//parse title id
	titleId, _ := parseTitleIdFromFileName(file.Name())
	version, _ := parseVersionFromFileName(file.Name())

	if titleId == nil || version == nil {
		return nil, errors.New("unable to determine titileId / version")
	}

	return &switchfs.ContentMetaAttributes{TitleId: *titleId, Version: *version}, nil
}

func parseVersionFromFileName(fileName string) (*int, error) {
	res := versionRegex.FindStringSubmatch(fileName)
	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no version id found")
	}
	ver, err := strconv.Atoi(res[1])
	if err != nil {
		return nil, errors.New("failed to parse name - no version id found")
	}
	return &ver, nil
}

func parseTitleIdFromFileName(fileName string) (*string, error) {
	res := titleIdRegex.FindStringSubmatch(fileName)

	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no title id found")
	}
	titleId := strings.ToLower(res[1])
	return &titleId, nil
}

func ParseTitleNameFromFileName(fileName string) string {
	ind := strings.Index(fileName, "[")
	if ind != -1 {
		return fileName[:ind]
	}
	return fileName
}
