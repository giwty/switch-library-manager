package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/jedib0t/go-pretty/table"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"switch/nsp-update/db"
	"switch/nsp-update/process"
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
	mode      = flag.String("m", "u", "mode (available options: (u) find missing updates / (dlc) find missing dlc / (d) delete old updates / (o) - organize library ")
	s         = spinner.New(spinner.CharSets[26], 100*time.Millisecond)
)

func main() {
	flag.Parse()

	//no folder was specified
	if *nspFolder == "" {
		flag.Usage()
		os.Exit(1)
	}

	versionsEtag := ""
	titlesEtag := ""

	//read config file
	if _, err := os.Stat(CONFIG_FILENAME); err == nil {
		file, err := os.Open("./nu_config.json")
		if err != nil {
			log.Print("Missing or corrupted config file, ignoring")
		} else {
			configMap := map[string]string{}
			err = json.NewDecoder(file).Decode(&configMap)
			versionsEtag = configMap["versions_etag"]
			titlesEtag = configMap["titles_etag"]
		}
	}

	//1. load the titles JSON object
	s.Start()
	fmt.Printf("Downlading latest switch titles json file\n")
	titleFile, titlesEtag, err := db.LoadAndUpdateFile(TITLES_JSON_URL, TITLEDB_FILENAME, titlesEtag)
	if err != nil {
		fmt.Printf("titleAttributes json file doesn't exist\n")
		return
	}

	//2. load the versions JSON object
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(VERSIONS_JSON_URL, VERSIONSDB_FILENAME, versionsEtag)
	if err != nil {
		fmt.Printf("titleAttributes json file doesn't exist\n")
		return
	}
	s.Stop()

	//3. update the config file with new etag
	etagMap := map[string]string{"versions_etag": versionsEtag, "titles_etag": titlesEtag}
	file, _ := json.MarshalIndent(etagMap, "", " ")
	_ = ioutil.WriteFile(CONFIG_FILENAME, file, 0644)

	//4. create switch titleAttributes db
	titlesDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)

	//5. read local files
	s.Restart()
	fmt.Printf("\nScanning nsp folder\n ")
	files, err := ioutil.ReadDir(*nspFolder)
	if err != nil {
		fmt.Printf("\nfailed accessing NSP folder\n %v", err)
		return
	}
	s.Stop()

	localDB, err := db.CreateLocalSwitchFilesDB(files, *nspFolder, *recursive)

	p := (float32(len(localDB.TitlesMap)) / float32(len(titlesDB.TitlesMap))) * 100

	fmt.Printf("Local library completion status: %.2f%% (have %d titles, out of %d titles)\n\n", p, len(localDB.TitlesMap), len(titlesDB.TitlesMap))

	if mode == nil || *mode == "u" {
		processMissingUpdates(localDB, titlesDB)
	}

	if mode != nil && *mode == "dlc" {
		processMissingDLC(localDB, titlesDB)
	}

	if mode != nil && *mode == "d" {
		process.DeleteOldUpdates(localDB, titlesDB)
	}

	if mode != nil && *mode == "o" {
		process.OrganizeByFolders(*nspFolder, localDB, titlesDB)
	}

	fmt.Printf("Completed")
}

func processMissingUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {
	incompleteTitles := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound available updates:\n\n")
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Local version", "Latest Version", "Update Date"})
	i := 0
	for _, v := range incompleteTitles {
		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, v.LatestUpdateDate})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()
}

func processMissingDLC(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {
	incompleteTitles := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound missing DLCS:\n\n")
	} else {
		fmt.Print("\nYou have all the DLCS!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs (titleId - Name)"})
	i := 0
	for _, v := range incompleteTitles {
		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, strings.Join(v.MissingDLC, "\n")})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()
}
