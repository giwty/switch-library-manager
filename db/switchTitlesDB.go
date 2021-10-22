package db

import (
	"encoding/json"
	"io"
	"strings"
)

const (
	TITLE_ID_BITMASK  = 0xFFFFFFFFFFFFE000
	TITLE_TYPE_BASE   = 0
	TITLE_TYPE_UPDATE = 800
)

type TitleAttributes struct {
	intId       uint64      `json:"-"`
	Id          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Version     json.Number `json:"version,omitempty"`
	Region      string      `json:"region,omitempty"`
	ReleaseDate int         `json:"releaseDate,omitempty"`
	Publisher   string      `json:"publisher,omitempty"`
	IconUrl     string      `json:"iconUrl,omitempty"`
	Screenshots []string    `json:"screenshots,omitempty"`
	BannerUrl   string      `json:"bannerUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Size        int         `json:"size,omitempty"`
}

type SwitchTitle struct {
	Attributes TitleAttributes
	Updates    map[int]string
	Dlc        map[string]TitleAttributes
}

type SwitchTitlesDB struct {
	TitlesMap map[string]*SwitchTitle
}

func CreateSwitchTitleDB(titlesFile, versionsFile io.Reader) (*SwitchTitlesDB, error) {
	//parse the titles objects
	var titles = map[string]TitleAttributes{}
	err := decodeToJsonObject(titlesFile, &titles)
	if err != nil {
		return nil, err
	}

	//parse the titles objects
	//titleID -> versionId-> release date
	var versions = map[string]map[int]string{}
	err = decodeToJsonObject(versionsFile, &versions)
	if err != nil {
		return nil, err
	}

	result := SwitchTitlesDB{TitlesMap: map[string]*SwitchTitle{}}
	for id, attr := range titles {
		// Grab the base ID and type
		baseId, titleType, err := getTitleBaseAndType(attr.Id)
		if err != nil {
			continue
		}

		// Try to grab the title from the result
		switchTitle, ok := result.TitlesMap[baseId]
		// If it does not exist, create the entry
		if !ok {
			result.TitlesMap[baseId] = &SwitchTitle{
				Dlc:     make(map[string]TitleAttributes),
				Updates: make(map[int]string),
			}
			switchTitle = result.TitlesMap[baseId]
		}

		// Process depending on type
		switch titleType {
		// Base game
		case TITLE_TYPE_BASE:
			switchTitle.Attributes = attr

		// Update
		case TITLE_TYPE_UPDATE:
			if titleUpdate, ok := versions[baseId]; ok {
				switchTitle.Updates = titleUpdate
			}

		// Otherwise a DLC
		default:
			switchTitle.Dlc[strings.ToLower(id)] = attr
		}
	}

	return &result, nil
}
