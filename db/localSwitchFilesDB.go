package db

import (
	"errors"
	//"github.com/dgraph-io/badger/v2"
	//	badger "github.com/dgraph-io/badger/v2"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/switchfs"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	versionRegex = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})]`)
	titleIdRegex = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z0-9]{16})]`)
	total        = 0
	globalInd    = 0
)

type LocalSwitchDBManager struct {
	//db *badger.DB
}

func NewLocalSwitchDBManager(baseFolder string) (*LocalSwitchDBManager, error) {
	// Open the Badger database located in the /tmp/badger directory.
	// It will be created if it doesn't exist.
	/*db, err := badger.Open(badger.DefaultOptions(baseFolder))
	if err != nil {
		log.Fatal(err)
	}*/

	return &LocalSwitchDBManager{}, nil
}

func (ldb *LocalSwitchDBManager) Close() {
	//ldb.db.Close()
}

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

func (sf *SwitchFile) String() string {
	var sb strings.Builder
	if sf.BaseExist {
		sb.WriteString("base:")
		sb.WriteString(sf.File.Info.Name())
		sb.WriteString("\n")
	}
	if sf.Updates != nil && len(sf.Updates) != 0 {
		sb.WriteString("Updates:")
		for _, update := range sf.Updates {
			sb.WriteString(update.Info.Name())
			sb.WriteString("\n")
		}
	}
	if sf.Dlc != nil && len(sf.Dlc) != 0 {
		sb.WriteString("Dlc:")
		for _, dlc := range sf.Dlc {
			sb.WriteString(dlc.Info.Name())
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type LocalSwitchFilesDB struct {
	TitlesMap map[string]*SwitchFile
	Skipped   map[os.FileInfo]string
}

func (ldb *LocalSwitchDBManager) CreateLocalSwitchFilesDB(files []os.FileInfo, parentFolder string, progress ProgressUpdater, recursive bool) (*LocalSwitchFilesDB, error) {
	titles := map[string]*SwitchFile{}
	skipped := map[os.FileInfo]string{}
	globalInd = 0
	total = 0
	ldb.scanLocalFiles(parentFolder, files, progress, recursive, titles, skipped)

	return &LocalSwitchFilesDB{TitlesMap: titles, Skipped: skipped}, nil
}

func (ldb *LocalSwitchDBManager) scanLocalFiles(parentFolder string, files []os.FileInfo,
	progress ProgressUpdater,
	recurse bool, titles map[string]*SwitchFile,
	skipped map[os.FileInfo]string) {
	total += len(files)
	for _, file := range files {
		globalInd += 1
		if progress != nil {
			progress.UpdateProgress(globalInd, total, file.Name())
		}
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
				zap.S().Errorf("failed scanning NSP folder [%v]", err)
				continue
			}
			ldb.scanLocalFiles(folder, innerFiles, progress, recurse, titles, skipped)
			continue
		}

		//only handle NSZ and NSP files
		if !strings.HasSuffix(strings.ToLower(file.Name()), "xci") &&
			!strings.HasSuffix(strings.ToLower(file.Name()), "nsp") &&
			!strings.HasSuffix(strings.ToLower(file.Name()), "nsz") &&
			!strings.HasSuffix(strings.ToLower(file.Name()), "xcz") {
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
			metadata.Type = "Update"
			if update, ok := switchTitle.Updates[metadata.Version]; ok {
				zap.S().Warnf("-->Duplicate update file found [%v] and [%v]", update.Info.Name(), file.Name())
			}
			switchTitle.Updates[metadata.Version] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
			continue
		}

		//process base
		if strings.HasSuffix(metadata.TitleId, "000") {
			metadata.Type = "Base"
			if switchTitle.BaseExist {
				zap.S().Warnf("-->Duplicate base file found [%v] and [%v]", file.Name(), switchTitle.File.Info.Name())
			}
			switchTitle.File = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
			switchTitle.BaseExist = true

			//handle XCI
			if metadata.Version != 0 {
				metadata.Type = "Update"
				switchTitle.Updates[metadata.Version] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
			}
			continue
		}

		if dlc, ok := switchTitle.Dlc[metadata.TitleId]; ok {
			zap.S().Warnf("-->Duplicate DLC file found [%v] and [%v]", file.Name(), dlc.Info.Name())
			if dlc.Metadata.Version > metadata.Version {
				continue
			}
		}
		//not an update, and not main TitleAttributes, so treat it as a DLC
		metadata.Type = "DLC"
		switchTitle.Dlc[metadata.TitleId] = ExtendedFileInfo{Info: file, BaseFolder: parentFolder, Metadata: metadata}
	}

}

func GetGameMetadata(file os.FileInfo, filePath string) (*switchfs.ContentMetaAttributes, error) {

	var metadata *switchfs.ContentMetaAttributes = nil
	keys, _ := settings.SwitchKeys()
	var err error

	if keys != nil && keys.GetKey("header_key") != "" {
		if strings.HasSuffix(strings.ToLower(file.Name()), "nsp") ||
			strings.HasSuffix(strings.ToLower(file.Name()), "nsz") {
			metadata, err = switchfs.ReadNspMetadata(filePath)
			if err != nil {
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.Name(), err)
			}
		} else if strings.HasSuffix(strings.ToLower(file.Name()), "xci") ||
			strings.HasSuffix(strings.ToLower(file.Name()), "xcz") {
			metadata, err = switchfs.ReadXciMetadata(filePath)
			if err != nil {
				zap.S().Errorf("[file:%v] failed to read file [reason: %v]\n", file.Name(), err)
			}
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
