package db

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/switchfs"
	"go.uber.org/zap"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	. "time"
)

var (
	versionRegex = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})]`)
	titleIdRegex = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z0-9]{16})]`)
	total        = 0
	globalInd    = 0
)

type LocalSwitchDBManager struct {
	db *bolt.DB
}

func NewLocalSwitchDBManager(baseFolder string) (*LocalSwitchDBManager, error) {
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open(filepath.Join(baseFolder, "slm.db"), 0600, &bolt.Options{Timeout: 1 * Second})
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return &LocalSwitchDBManager{db: db}, nil
}

func (ldb *LocalSwitchDBManager) Close() {
	ldb.db.Close()
}

type ExtendedFileInfo struct {
	Info       os.FileInfo
	BaseFolder string
	Metadata   *switchfs.ContentMetaAttributes
}

type SwitchFile struct {
	File         ExtendedFileInfo
	BaseExist    bool
	Updates      map[int]ExtendedFileInfo
	Dlc          map[string]ExtendedFileInfo
	MultiContent bool
	LatestUpdate int
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

		contentMap, err := ldb.getGameMetadata(file, filePath)

		for _, metadata := range contentMap {

			if err != nil {
				skipped[file] = "unable to determine title-Id / version"
				continue
			}

			idPrefix := metadata.TitleId[0 : len(metadata.TitleId)-4]

			multiContent := len(contentMap) > 1
			switchTitle := &SwitchFile{
				MultiContent: multiContent,
				Updates:      map[int]ExtendedFileInfo{},
				Dlc:          map[string]ExtendedFileInfo{},
				BaseExist:    false,
				LatestUpdate: 0,
			}
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
				if metadata.Version > switchTitle.LatestUpdate {
					switchTitle.LatestUpdate = metadata.Version
				}
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

}

func (ldb *LocalSwitchDBManager) getGameMetadata(file os.FileInfo, filePath string) (map[string]*switchfs.ContentMetaAttributes, error) {

	var metadata map[string]*switchfs.ContentMetaAttributes = nil
	keys, _ := settings.SwitchKeys()
	var err error
	fileKey := filePath + "|" + file.Name() + "|" + strconv.Itoa(int(file.Size()))
	if keys != nil && keys.GetKey("header_key") != "" {

		err = ldb.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("deep-scan"))
			if b == nil {
				return nil
			}
			v := b.Get([]byte(fileKey))
			if v == nil {
				return nil
			}
			d := gob.NewDecoder(bytes.NewReader(v))

			// Decoding the serialized data
			err = d.Decode(&metadata)
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			zap.S().Warnf("%v", err)
		}

		if metadata != nil {
			return metadata, nil
		}

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
		err = ldb.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("deep-scan"))
			if b == nil {
				b, err = tx.CreateBucket([]byte("deep-scan"))
				if b == nil || err != nil {
					return fmt.Errorf("create bucket: %s", err)
				}
			}
			var bytesBuff bytes.Buffer
			encoder := gob.NewEncoder(&bytesBuff)
			err = encoder.Encode(metadata)
			if err != nil {
				return err
			}
			err := b.Put([]byte(fileKey), bytesBuff.Bytes())
			return err
		})
		if err != nil {
			zap.S().Warnf("%v", err)
		}
		return metadata, nil
	}

	//fallback to parse data from filename

	//parse title id
	titleId, _ := parseTitleIdFromFileName(file.Name())
	version, _ := parseVersionFromFileName(file.Name())

	if titleId == nil || version == nil {
		return nil, errors.New("unable to determine titileId / version")
	}
	metadata = map[string]*switchfs.ContentMetaAttributes{}
	metadata[*titleId] = &switchfs.ContentMetaAttributes{TitleId: *titleId, Version: *version}

	return metadata, nil
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
