package db

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/giwty/switch-library-manager/fileio"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/switchfs"
	"go.uber.org/zap"
)

// Local database manager
type LocalSwitchDBManager struct {
	db       *PersistentDB
	settings *settings.AppSettings
	logger   *zap.SugaredLogger
}

// Constructor for the local database manager
func NewLocalSwitchDBManager(baseFolder string, l *zap.SugaredLogger, s *settings.AppSettings) *LocalSwitchDBManager {
	return &LocalSwitchDBManager{
		db:       NewPersistentDB(baseFolder, l, s),
		settings: s,
		logger:   l,
	}
}

func (ldb *LocalSwitchDBManager) Close() {
	ldb.db.Close()
}

func (ldb *LocalSwitchDBManager) CreateLocalSwitchFilesDB(folders []string,
	progress ProgressUpdater, recursive bool, ignoreCache bool) (*LocalSwitchFilesDB, error) {

	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	files := []ExtendedFileInfo{}

	if !ignoreCache {
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "files", &files)
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "skipped", &skipped)
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "titles", &titles)
	}

	if len(titles) == 0 {

		for i, folder := range folders {
			err := scanFolder(folder, recursive, &files, progress)
			if progress != nil {
				progress.UpdateProgress(i+1, len(folders)+1, "scanning files in "+folder)
			}
			if err != nil {
				continue
			}
		}

		ldb.processLocalFiles(files, progress, titles, skipped)

		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "files", files)
		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "skipped", skipped)
		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "titles", titles)
	}

	if progress != nil {
		progress.UpdateProgress(len(files), len(files), "Complete")
	}

	return &LocalSwitchFilesDB{TitlesMap: titles, Skipped: skipped, NumFiles: len(files)}, nil
}

func (ldb *LocalSwitchDBManager) ClearScanData() error {
	return ldb.db.ClearTable(DB_TABLE_FILE_SCAN_METADATA)
}

func (ldb *LocalSwitchDBManager) processLocalFiles(files []ExtendedFileInfo,
	progress ProgressUpdater,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile) {
	ind := 0
	total := len(files)
	for _, file := range files {
		ind += 1
		if progress != nil {
			progress.UpdateProgress(ind, total, "process:"+file.FileName)
		}

		//scan sub-folders if flag is present
		filePath := filepath.Join(file.BaseFolder, file.FileName)
		if file.IsDir {
			continue
		}

		fileName := strings.ToLower(file.FileName)
		isSplit := false

		if partNum, err := strconv.Atoi(fileName[len(fileName)-2:]); err == nil {
			if partNum == 0 {
				isSplit = true
			} else {
				continue
			}

		}

		//only handle NSZ and NSP files

		if !isSplit &&
			!strings.HasSuffix(fileName, "xci") &&
			!strings.HasSuffix(fileName, "nsp") &&
			!strings.HasSuffix(fileName, "nsz") &&
			!strings.HasSuffix(fileName, "xcz") {
			skipped[file] = SkippedFile{ReasonCode: REASON_UNSUPPORTED_TYPE, ReasonText: "file type is not supported"}
			continue
		}

		contentMap, err := ldb.getGameMetadata(file, filePath, skipped)

		if err != nil {
			if _, ok := skipped[file]; !ok {
				skipped[file] = SkippedFile{ReasonText: "unable to determine title-Id / version - " + err.Error(), ReasonCode: REASON_UNRECOGNISED}
			}
			continue
		}

		for _, metadata := range contentMap {
			// Grab the base ID and type
			baseId, titleType, err := getTitleBaseAndType(metadata.TitleId)
			if err != nil {
				continue
			}

			multiContent := len(contentMap) > 1
			switchTitle := &SwitchGameFiles{
				MultiContent: multiContent,
				Updates:      map[int]SwitchFileInfo{},
				Dlc:          map[string]SwitchFileInfo{},
				BaseExist:    false,
				IsSplit:      isSplit,
				LatestUpdate: 0,
			}
			if t, ok := titles[baseId]; ok {
				switchTitle = t
			}
			titles[baseId] = switchTitle

			// Process depending on type
			switch titleType {
			// Update
			case TITLE_TYPE_UPDATE:
				metadata.Type = "Update"

				if update, ok := switchTitle.Updates[metadata.Version]; ok {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate update file (" + update.ExtendedInfo.FileName + ")"}
					zap.S().Warnf("-->Duplicate update file found [%v] and [%v]", update.ExtendedInfo.FileName, file.FileName)
					continue
				}
				switchTitle.Updates[metadata.Version] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				if metadata.Version > switchTitle.LatestUpdate {
					if switchTitle.LatestUpdate != 0 {
						skipped[switchTitle.Updates[switchTitle.LatestUpdate].ExtendedInfo] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally"}
					}
					switchTitle.LatestUpdate = metadata.Version
				} else {
					skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally"}
				}
				continue

			// Base game
			case TITLE_TYPE_BASE:
				metadata.Type = "Base"
				if switchTitle.BaseExist {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate base file (" + switchTitle.File.ExtendedInfo.FileName + ")"}
					zap.S().Warnf("-->Duplicate base file found [%v] and [%v]", file.FileName, switchTitle.File.ExtendedInfo.FileName)
					continue
				}
				switchTitle.File = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				switchTitle.BaseExist = true

				continue

			// Otherwise a DLC
			default:
				if dlc, ok := switchTitle.Dlc[metadata.TitleId]; ok {
					if metadata.Version < dlc.Metadata.Version {
						skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old DLC file, newer version exist locally"}
						zap.S().Warnf("-->Old DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
						continue
					} else if metadata.Version == dlc.Metadata.Version {
						skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate DLC file (" + dlc.ExtendedInfo.FileName + ")"}
						zap.S().Warnf("-->Duplicate DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
						continue
					}
				}
				//not an update, and not main TitleAttributes, so treat it as a DLC
				metadata.Type = "DLC"
				switchTitle.Dlc[metadata.TitleId] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
			}
		}
	}
}

func (ldb *LocalSwitchDBManager) getGameMetadata(file ExtendedFileInfo,
	filePath string,
	skipped map[ExtendedFileInfo]SkippedFile) (map[string]*switchfs.ContentMetaAttributes, error) {

	var metadata map[string]*switchfs.ContentMetaAttributes = nil
	var err error
	fileKey := filePath + "|" + file.FileName + "|" + strconv.Itoa(int(file.Size))
	if ldb.settings.GetKey(settings.SETTINGS_PRODKEYS_HEADER) != "" {
		err = ldb.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, &metadata)

		if err != nil {
			zap.S().Warnf("%v", err)
		}

		if metadata != nil {
			return metadata, nil
		}

		fileName := strings.ToLower(file.FileName)
		if strings.HasSuffix(fileName, "nsp") ||
			strings.HasSuffix(fileName, "nsz") {
			metadata, err = switchfs.ReadNspMetadata(filePath, ldb.settings.SwitchKeys)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "xci") ||
			strings.HasSuffix(fileName, "xcz") {
			metadata, err = switchfs.ReadXciMetadata(filePath, ldb.settings.SwitchKeys)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read file [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "00") {
			metadata, err = fileio.ReadSplitFileMetadata(filePath, ldb.settings.SwitchKeys)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read split files [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		}
	}

	if metadata != nil {
		err = ldb.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, metadata)

		if err != nil {
			zap.S().Warnf("%v", err)
		}
		return metadata, nil
	}

	//fallback to parse data from filename

	//parse title id
	titleId, _ := parseTitleIdFromFileName(file.FileName)
	version, _ := parseVersionFromFileName(file.FileName)

	if titleId == nil || version == nil {
		return nil, errors.New("unable to determine titileId / version")
	}
	metadata = map[string]*switchfs.ContentMetaAttributes{}
	metadata[*titleId] = &switchfs.ContentMetaAttributes{TitleId: *titleId, Version: *version}

	return metadata, nil
}
