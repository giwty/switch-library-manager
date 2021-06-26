package db

import "path/filepath"

type SwitchGameFiles struct {
	File         SwitchFileInfo
	BaseExist    bool
	Updates      map[int]SwitchFileInfo
	Dlc          map[string]SwitchFileInfo
	MultiContent bool
	LatestUpdate int
	IsSplit      bool
}

func (sgf *SwitchGameFiles) Type() string {
	if sgf.IsSplit {
		return "split"
	}
	if sgf.MultiContent {
		return "multi-content"
	}
	ext := filepath.Ext(sgf.File.ExtendedInfo.FileName)
	if len(ext) > 1 {
		return ext[1:]
	}
	return ""
}
