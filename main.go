package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/jedib0t/go-pretty/table"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	TITLEDB_FILENAME    = "titlesDb.json"
	VERSIONSDB_FILENAME = "versionsDb.json"
	CONFIG_FILENAME     = "nu_config.json"
	TITLES_JSON_URL     = "https://tinfoil.media/repo/db/titles.json"
	VERSIONS_JSON_URL   = "https://tinfoil.media/repo/db/versions.json"
)

var (
	nspFolder = flag.String("f", "", "path to NSP folder")
	recursive = flag.Bool("r", false, "recursively scan sub folders")
	versionR  = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})\]`)
	titleIdR  = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z,0-9]{16})\]`)
	s         = spinner.New(spinner.CharSets[26], 100*time.Millisecond)
)

type title struct {
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

func main() {
	flag.Parse()

	//no folder was specified
	if *nspFolder == "" {
		flag.Usage()
		os.Exit(1)
	}

	currVersionEtag := ""
	currTitlesEtag := ""

	//read etag version from config file
	if _, err := os.Stat(CONFIG_FILENAME); err == nil {
		file, err := os.Open("./nu_config.json")
		if err != nil {
			log.Print("Failed to read config file, ignoring")
		} else {
			configMap := map[string]string{}
			err = json.NewDecoder(file).Decode(&configMap)
			currVersionEtag = configMap["versions_etag"]
			currTitlesEtag = configMap["titles_etag"]
		}
	}

	var titlesDb = map[string]title{}
	titlesEtag, err := loadOrDownloadFileFromUrl(TITLES_JSON_URL, TITLEDB_FILENAME, currTitlesEtag, &titlesDb)
	if err != nil {
		fmt.Printf("unable to download file - %v\n%v", TITLES_JSON_URL, err)
		return
	}

	var titlesPrefixDb = map[string][]title{}
	for _, t := range titlesDb {
		id := strings.ToLower(t.Id)
		prefix := id[0 : len(id)-4]
		if strings.HasSuffix(id, "800") {
			continue
		}
		titles, ok := titlesPrefixDb[prefix]
		if !ok {
			titles = []title{}
		}
		titles = append(titles, t)
		titlesPrefixDb[prefix] = titles
	}

	//titleID -> versionId-> release date
	var versionsDb = map[string]map[int]string{}
	versionsEtag, err := loadOrDownloadFileFromUrl(VERSIONS_JSON_URL, VERSIONSDB_FILENAME, currVersionEtag, &versionsDb)
	if err != nil {
		fmt.Printf("unable to download file - %v\n%v", VERSIONS_JSON_URL, err)
		return
	}

	if versionsEtag != currVersionEtag || titlesEtag != currTitlesEtag {
		etagMap := map[string]string{"versions_etag": versionsEtag, "titles_etag": titlesEtag}
		file, _ := json.MarshalIndent(etagMap, "", " ")
		_ = ioutil.WriteFile(CONFIG_FILENAME, file, 0644)
	}

	s.Restart()
	fmt.Printf("\nScanning nsp folder ")
	files, err := ioutil.ReadDir(*nspFolder)
	if err != nil {
		fmt.Printf("\nfailed scanning NSP folder\n %v", err)
		return
	}
	s.Stop()
	var localVersionsDb = map[string][]int{}
	var skippedFiles = map[string]string{}

	scanLocalFiles(*nspFolder, files, *recursive, localVersionsDb, skippedFiles)

	var numTobeUpdated int = 0
	type Dlcs struct {
		dlcs  []string
		title string
	}
	missingDLC := map[string]Dlcs{}
	fmt.Print("Missing Updates:\n\n")
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Current version", "Available Version", "Release date"})
	//iterate over local files, and compare to remote versions
	for titleId, _ := range localVersionsDb {

		localVersions := localVersionsDb[titleId]
		sort.Ints(localVersions)

		id := strings.ToLower(titleId)
		prefix := id[0 : len(id)-4]
		//find missing DLC's
		//do this once per title id
		if _, ok := missingDLC[prefix]; !ok {
			dlcsObj := Dlcs{}
			dlcsObj.dlcs = []string{}
			for _, t := range titlesPrefixDb[prefix] {
				//check if titleid exist in local db
				if strings.HasSuffix(t.Id, "000") {
					dlcsObj.title = t.Name
				}
				if _, ok := localVersionsDb[strings.ToLower(t.Id)]; !ok {
					name := strings.ToLower(t.Name)
					//if !strings.Contains(name,"audio") && !    strings.Contains(name,"language") {
					dlcsObj.dlcs = append(dlcsObj.dlcs, t.Id+" ["+name+"] ")
					//	}
				}
			}
			if len(dlcsObj.dlcs) != 0 {
				missingDLC[prefix] = dlcsObj
			}

		}

		var remoteVersions []int
		for k, _ := range versionsDb[titleId] {
			remoteVersions = append(remoteVersions, k)
		}

		if title, ok := titlesDb[strings.ToUpper(titleId[0:len(titleId)-3]+"800")]; ok {
			ver, err := strconv.Atoi(title.Version.String())
			if err == nil {
				remoteVersions = append(remoteVersions, ver)
			}
		}
		if len(remoteVersions) == 0 {
			continue
		}

		var nspName string
		if title, ok := titlesDb[strings.ToUpper(titleId)]; ok {
			nspName = title.Name
			ver, err := strconv.Atoi(title.Version.String())
			if err == nil {
				remoteVersions = append(remoteVersions, ver)
			}
		}

		localVer := localVersions[len(localVersions)-1]
		if localVersions[0] != 0 && !strings.Contains(strings.ToLower(nspName), "pack") {
			//fmt.Printf("** game [%v][%v] missing base version\n",titleId,nspName)
			continue
		}
		sort.Ints(remoteVersions)
		remoteVer := remoteVersions[len(remoteVersions)-1]

		if remoteVer > localVer {
			var nspName string
			if title, ok := titlesDb[strings.ToUpper(titleId)]; ok {
				nspName = title.Name
			}
			numTobeUpdated++
			t.AppendRow([]interface{}{numTobeUpdated, nspName, titleId, localVer, remoteVer, versionsDb[titleId][remoteVer]})
		}
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", numTobeUpdated})
	if numTobeUpdated != 0 {
		fmt.Print("\nFound available updates:\n\n")
		t.Render()
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
	}
	fmt.Print("\n\nMissing DLCs\n\n")
	t = table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs (titleId - Name)"})
	numMissingDlcs := 0
	for titleId, dlcs := range missingDLC {
		numMissingDlcs++
		t.AppendRow([]interface{}{numMissingDlcs, dlcs.title, titleId, strings.Join(dlcs.dlcs, "\n")})
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", numMissingDlcs})
	t.Render()
}

func scanLocalFiles(path string, files []os.FileInfo, recurse bool, localVersionsDb map[string][]int, skippedFiles map[string]string) {

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
			folder := filepath.Join(path, file.Name())
			innerFiles, err := ioutil.ReadDir(folder)
			if err != nil {
				fmt.Printf("\nfailed scanning NSP folder\n %v", err)
				continue
			}
			scanLocalFiles(folder, innerFiles, recurse, localVersionsDb, skippedFiles)
		}

		//only handle NSZ and NSP files
		if !strings.HasSuffix(file.Name(), "nsp") && !strings.HasSuffix(file.Name(), "nsz") {
			skippedFiles[file.Name()] = "non NSP file"
			continue
		}

		res := versionR.FindStringSubmatch(file.Name())
		if len(res) != 2 {
			skippedFiles[file.Name()] = "failed to parse name"
			continue
		}
		verStr := res[1]
		res = titleIdR.FindStringSubmatch(file.Name())
		if len(res) != 2 {
			skippedFiles[file.Name()] = "failed to parse name"
			continue
		}
		if len(res) != 2 {
			skippedFiles[file.Name()] = "failed to parse name"
			continue
		}
		titleId := strings.ToLower(res[1])

		if strings.HasSuffix(titleId, "800") {
			titleId = titleId[0:len(titleId)-3] + "000"
		}

		ver, err := strconv.Atoi(verStr)
		if err != nil {
			skippedFiles[file.Name()] = "failed to parse version"
			continue
		}

		localVersionsDb[titleId] = append(localVersionsDb[titleId], ver)
	}
}

func loadOrDownloadFileFromUrl(url string, fileName string, etag string, target interface{}) (string, error) {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return etag, err
	}
	req.Header.Set("If-None-Match", etag)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return etag, err
	}

	if resp.StatusCode >= 400 {
		return etag, errors.New("got a non 200 response - " + resp.Status)
	}
	defer resp.Body.Close()
	//getting the new etag
	newEtag := resp.Header.Get("Etag")
	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("\nCreating/Updating [%v] from - [%v]", fileName, url)
		err := os.Rename("./"+fileName, "./"+fileName+".bak")
		if err != nil {
			fmt.Print("\n (local file doesnt exist)")
		}
		s.Start()
		body, err := ioutil.ReadAll(resp.Body)
		err = ioutil.WriteFile("./"+fileName, body, 0644)
		if err != nil {
			return etag, err
		}
		//continue to load the file
		fallthrough
	case http.StatusNotModified:
		fmt.Printf("\nLoading file - %v", fileName)
		s.Start()
		file, err := os.Open("./" + fileName)
		err = json.NewDecoder(file).Decode(target)
		if err != nil {
			fmt.Print("\nNew file version is malformed, trying to load previous version")
			//try to restore the last known working file
			err := os.Remove("./" + fileName)
			err = os.Rename("./"+fileName+".bak", "./"+fileName)
			if err != nil {
				fmt.Print("\n (No backup, or failed to load backup)")
				return etag, err
			}
			err = json.NewDecoder(file).Decode(target)
			return etag, err
		}
	}

	return newEtag, nil
}
