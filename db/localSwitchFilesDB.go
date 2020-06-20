package db

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	versionRegex = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})\]`)
	titleIdRegex = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z,0-9]{16})\]`)
)

type ExtendedFileInfo struct {
	Info       os.FileInfo
	BaseFolder string
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

func scanLocalFiles(parentFolder string, files []os.FileInfo, recurse bool, titles map[string]*SwitchFile, skipped map[os.FileInfo]string) {

	for _, file := range files {
		//skip mac hidden files
		if file.Name()[0:1] == "." {
			continue
		}

		//scan sub-folders if flag is present
		if file.IsDir() {
			if !recurse {
				continue
			}
			folder := filepath.Join(parentFolder, file.Name())
			innerFiles, err := ioutil.ReadDir(folder)
			if err != nil {
				fmt.Printf("\nfailed scanning NSP folder\n %v", err)
				continue
			}
			scanLocalFiles(folder, innerFiles, recurse, titles, skipped)
		}

		//only handle NSZ and NSP files
		if !strings.HasSuffix(file.Name(), "nsp") && !strings.HasSuffix(file.Name(), "nsz") {
			skipped[file] = "non NSP File"
			continue
		}

		//parse title id
		res := titleIdRegex.FindStringSubmatch(file.Name())
		if len(res) != 2 {
			skipped[file] = "failed to parse name - no title id found"
			continue
		}
		titleId := strings.ToLower(res[1])

		//parse version id
		res = versionRegex.FindStringSubmatch(file.Name())
		if len(res) != 2 {
			skipped[file] = "failed to parse name - no version id found"
			continue
		}
		ver, err := strconv.Atoi(res[1])
		if err != nil {
			skipped[file] = "failed to parse version - unable to parse version id"
			continue
		}

		idPrefix := titleId[0 : len(titleId)-4]
		switchTitle := &SwitchFile{Updates: map[int]ExtendedFileInfo{}, Dlc: map[string]ExtendedFileInfo{}, BaseExist: false}
		if t, ok := titles[idPrefix]; ok {
			switchTitle = t
		}
		titles[idPrefix] = switchTitle

		//process Updates
		if strings.HasSuffix(titleId, "800") {
			switchTitle.Updates[ver] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder}
			continue
		}

		//process main TitleAttributes
		if strings.HasSuffix(titleId, "000") {
			switchTitle.File = ExtendedFileInfo{Info: file, BaseFolder: parentFolder}
			switchTitle.BaseExist = true
			continue
		}

		//not an update, and not main TitleAttributes, so treat it as a DLC
		switchTitle.Dlc[titleId] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder}
	}
}
