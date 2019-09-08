package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/jedib0t/go-pretty/table"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	titles_json_uri   = "https://raw.githubusercontent.com/blawar/titledb/master/titles.US.en.json"
	versions_json_url = "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
)

var (
	nspFolder = flag.String("f", "", "path to NSP folder")
	s         = spinner.New(spinner.CharSets[26], 100*time.Millisecond) // Build our new spinner
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

	if *nspFolder == "" {
		flag.Usage()
		os.Exit(1)
	}

	var titlesDb = map[string]title{}
	err := loadOrDownloadFileFromUrl(titles_json_uri, "titlesDb.json", &titlesDb)
	if err != nil {
		fmt.Printf("unable to download file - %v\n%v", titles_json_uri, err)
		return
	}

	var versionsDb = map[string]map[int]string{}
	err = loadOrDownloadFileFromUrl(versions_json_url, "versionsDb.json", &versionsDb)
	if err != nil {
		fmt.Printf("unable to download file - %v\n%v", versions_json_url, err)
		return
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

	versionR := regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})\]`)
	titleIdR := regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z,0-9]{16})\]`)
	for _, file := range files {
		if file.Name()[0:1] == "." || file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), "nsp") {
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

	var numTobeUpdated int = 0

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title","TitleId" ,"Current version", "Available Version", "Release date"})
	//iterate over local files, and compare to remote versions
	for titleId, _ := range localVersionsDb {

		localVersions := localVersionsDb[titleId]
		sort.Ints(localVersions)

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
			t.AppendRow([]interface{}{numTobeUpdated, nspName,titleId, localVer, remoteVer, versionsDb[titleId][remoteVer]})
		}
	}
	t.AppendFooter(table.Row{"", "", "", "","Total", numTobeUpdated})
	if numTobeUpdated != 0{
		fmt.Printf("\nFound available updates:\n\n")
		t.Render()
	}else{
		fmt.Printf("\nAll NSP's are up to date!\n\n")
	}


}

func loadOrDownloadFileFromUrl(url string, fileName string, target interface{}) error {

	if _, err := os.Stat("./" + fileName); os.IsNotExist(err) {
		file, err := os.Create("./" + fileName)
		fmt.Printf("\nDownloading from - %v", url)
		s.Start()
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return errors.New("got a non 200 response - " + resp.Status)
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return err
		}
	}

	fmt.Printf("\nLoading file - %v", fileName)
	s.Start()
	file, err := os.Open("./" + fileName)
	err = json.NewDecoder(file).Decode(target)
	if err != nil {
		return err
	}
	return nil
}
