package consoleapp

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/schollz/progressbar/v3"
	"github.com/trembon/switch-library-manager/backend/console"
	"github.com/trembon/switch-library-manager/backend/dateformat"
	"github.com/trembon/switch-library-manager/backend/db"
	"github.com/trembon/switch-library-manager/backend/export"
	"github.com/trembon/switch-library-manager/backend/process"
	"github.com/trembon/switch-library-manager/backend/settings"
	"go.uber.org/zap"
)

var (
	progressBar *progressbar.ProgressBar
)

const (
	csvGamesKind          = "games"
	csvMissingGamesKind   = "missing_games"
	csvMissingUpdatesKind = "missing_updates"
	csvMissingDLCKind     = "missing_dlc"
	csvIssuesKind         = "issues"
)

func csvExportFilename(kind string, date time.Time) string {
	return fmt.Sprintf("slm_%s.%s.csv", kind, date.Format("2006-01-02"))
}

type Console struct {
	baseFolder   string
	sugarLogger  *zap.SugaredLogger
	consoleFlags *console.ConsoleFlags
	xlsxOutput   string
	exportDate   time.Time
}

func CreateConsole(baseFolder string, sugarLogger *zap.SugaredLogger, consoleFlags *console.ConsoleFlags) *Console {
	return &Console{baseFolder: baseFolder, sugarLogger: sugarLogger, consoleFlags: consoleFlags}
}

func (c *Console) Start() {
	settingsObj, err := settings.ReadSettings(c.baseFolder)
	if err != nil {
		fmt.Printf("Failed to load settings: %v\n", err)
		return
	}
	cache, err := settings.ReadCache(c.baseFolder)
	if err != nil {
		fmt.Printf("Failed to load cache: %v\n", err)
		return
	}

	// 0. prepare independent export destinations
	csvOutput := ""
	if c.consoleFlags.ExportCsv.IsSet() {
		csvOutput = c.prepareExportFolder(c.consoleFlags.ExportCsv.String(), "CSV")
	}
	c.xlsxOutput = ""
	if c.consoleFlags.ExportXlsx.IsSet() {
		c.xlsxOutput = c.prepareExportFolder(c.consoleFlags.ExportXlsx.String(), "Excel")
	}
	exportDate := time.Now()
	c.exportDate = exportDate

	//1. load the titles JSON object
	fmt.Println("Downloading latest switch titles json file")
	progressBar = progressbar.New(2)

	filename := filepath.Join(c.baseFolder, settings.TITLE_JSON_FILENAME)
	titleFile, titleCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.TitlesURL, filename, db.RemoteFileCache{
		URL: cache.TitlesURL, ETag: cache.TitlesETag, SHA256: cache.TitlesSHA256,
	}, db.ValidateTitlesJSON)
	if err != nil {
		fmt.Printf("title json file doesn't exist\n")
		return
	}
	defer func() {
		if closeErr := titleFile.Close(); closeErr != nil {
			fmt.Printf("failed to close title json file: %v\n", closeErr)
			if c.sugarLogger != nil {
				c.sugarLogger.Errorf("failed to close title json file: %v", closeErr)
			}
		}
	}()
	cache.TitlesETag, cache.TitlesURL, cache.TitlesSHA256 = titleCache.ETag, titleCache.URL, titleCache.SHA256
	progressBar.Add(1)
	//2. load the versions JSON object
	filename = filepath.Join(c.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsCache, err := db.LoadAndUpdateFile(settingsObj.DataSources.VersionsURL, filename, db.RemoteFileCache{
		URL: cache.VersionsURL, ETag: cache.VersionsETag, SHA256: cache.VersionsSHA256,
	}, db.ValidateVersionsJSON)
	if err != nil {
		fmt.Printf("version json file doesn't exist\n")
		return
	}
	defer func() {
		if closeErr := versionsFile.Close(); closeErr != nil {
			fmt.Printf("failed to close version json file: %v\n", closeErr)
			if c.sugarLogger != nil {
				c.sugarLogger.Errorf("failed to close version json file: %v", closeErr)
			}
		}
	}()
	cache.VersionsETag, cache.VersionsURL, cache.VersionsSHA256 = versionsCache.ETag, versionsCache.URL, versionsCache.SHA256
	progressBar.Add(1)
	progressBar.Finish()
	newUpdate, err := settings.CheckForUpdates()

	if newUpdate {
		fmt.Printf("\n=== New version available, download from Github ===\n")
	}

	//3. update the internal cache with new etags
	if err := settings.SaveCacheWithError(cache, c.baseFolder); err != nil {
		fmt.Printf("Failed to save cache: %v\n", err)
		return
	}

	//4. create switch title db
	titlesDB, err := db.CreateSwitchTitleDB(titleFile, versionsFile)
	if err != nil {
		fmt.Printf("failed to create switch title database: %v\n", err)
		return
	}

	// 5. read local files
	folderToScan := settingsObj.Paths.LibraryFolder
	if c.consoleFlags.NspFolder.IsSet() && c.consoleFlags.NspFolder.String() != "" {
		folderToScan = c.consoleFlags.NspFolder.String()
	}

	if folderToScan == "" {
		fmt.Printf("\n\nNo folder to scan was defined, please edit settings.json with the folder path\n")
		return
	}
	fmt.Printf("\n\nScanning folder [%v]", folderToScan)
	progressBar = progressbar.New(2000)
	keys, _ := settings.InitSwitchKeys(c.baseFolder)
	if keys == nil || keys.GetKey("header_key") == "" {
		fmt.Printf("\n!!NOTE!!: keys file was not found, deep scan is disabled, library will be based on file tags.\n %v", err)
	}

	recursiveMode := settingsObj.Scan.Recursive
	if c.consoleFlags.Recursive.IsSet() {
		recursiveMode = c.consoleFlags.Recursive.Bool()
	}

	localDbManager, err := db.NewLocalSwitchDBManager(c.baseFolder)
	if err != nil {
		fmt.Printf("failed to create local files db :%v\n", err)
		return
	}
	defer localDbManager.Close()

	scanFolders := settingsObj.Paths.ScanFolders
	scanFolders = append(scanFolders, folderToScan)

	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, c, recursiveMode, true)
	if err != nil {
		fmt.Printf("\nfailed to process local folder\n %v", err)
		return
	}
	progressBar.Finish()

	p := (float32(len(localDB.TitlesMap)) / float32(len(titlesDB.TitlesMap))) * 100

	fmt.Printf("Local library completion status: %.2f%% (have %d titles, out of %d titles)\n", p, len(localDB.TitlesMap), len(titlesDB.TitlesMap))

	issuesCsvFile := ""
	if csvOutput != "" {
		issuesCsvFile = filepath.Join(csvOutput, csvExportFilename(csvIssuesKind, exportDate))
	}
	organizationEnabled := organizationEnabled(settingsObj.Organization)
	destructiveEnabled := settingsObj.Organization.DeleteOldUpdateFiles || organizationEnabled
	canMutate := true
	var organizationConflicts []process.OrganizationConflict
	var organizationPlan process.OrganizationPlan
	if organizationEnabled {
		organizationPlan, err = process.BuildOrganizationPlan(folderToScan, localDB, titlesDB, settingsObj.Organization)
		if err != nil {
			fmt.Printf("\nFailed to preflight library organization: %v\n", err)
			canMutate = false
		} else if len(organizationPlan.Conflicts) != 0 {
			organizationConflicts = organizationPlan.Conflicts
			fmt.Printf("\nOrganization canceled: %d destination conflict(s); see Issues\n", len(organizationConflicts))
			canMutate = false
		}
	}

	if canMutate && destructiveEnabled {
		if err := localDbManager.ClearLocalLibrarySnapshot(); err != nil {
			fmt.Printf("\nFailed to invalidate cached library before organization: %v\n", err)
			canMutate = false
		}
	}
	if canMutate && destructiveEnabled {
		operationErr := error(nil)
		if settingsObj.Organization.DeleteOldUpdateFiles {
			progressBar = progressbar.New(2000)
			fmt.Printf("\nDeleting verified duplicate and old files\n")
			operationErr = process.DeleteOldUpdatesAt(c.baseFolder, folderToScan, localDB, c)
			progressBar.Finish()
		}
		if operationErr == nil && organizationEnabled {
			progressBar = progressbar.New(2000)
			fmt.Printf("\nStarting library organization\n")
			operationErr = process.ExecuteOrganizationPlan(organizationPlan, localDB, folderToScan, settingsObj.Organization, c, settingsObj.Paths.ScanFolders...)
			progressBar.Finish()
		}
		if operationErr != nil {
			fmt.Printf("\nLibrary organization did not complete: %v\n", operationErr)
		}
		localDB, err = localDbManager.CreateLocalSwitchFilesDB(scanFolders, c, recursiveMode, true)
		if err != nil {
			fmt.Printf("\nFailed to refresh the local scan after organization: %v\n", err)
			return
		}
	}
	c.processIssues(localDB, issuesCsvFile, organizationConflicts)

	if settingsObj.MissingContent.CheckForUpdates {
		fmt.Printf("\nChecking for missing updates\n")

		missingUpdatesCsvFile := ""
		if csvOutput != "" {
			missingUpdatesCsvFile = filepath.Join(csvOutput, csvExportFilename(csvMissingUpdatesKind, exportDate))
		}

		c.processMissingUpdates(localDB, titlesDB, settingsObj, missingUpdatesCsvFile)
	}

	if settingsObj.MissingContent.CheckForDLC {
		fmt.Printf("\nChecking for missing DLC\n")

		missingDlcCsvFile := ""
		if csvOutput != "" {
			missingDlcCsvFile = filepath.Join(csvOutput, csvExportFilename(csvMissingDLCKind, exportDate))
		}

		c.processMissingDLC(localDB, titlesDB, settingsObj, missingDlcCsvFile)
	}

	fmt.Printf("Completed")
}

func organizationEnabled(options settings.OrganizationSettings) bool {
	return options.RenameFiles || options.CreateFolderPerGame || options.MoveScanFilesToLibrary
}

func (c *Console) processIssues(localDB *db.LocalSwitchFilesDB, csvOutput string, conflicts []process.OrganizationConflict) {
	type issueRow struct {
		file, reason, code string
	}
	rows := make([]issueRow, 0, len(localDB.Skipped)+len(conflicts))
	for file, skipped := range localDB.Skipped {
		rows = append(rows, issueRow{
			file:   filepath.Join(file.BaseFolder, file.FileName),
			reason: skipped.ReasonText,
			code:   strconv.Itoa(skipped.ReasonCode),
		})
	}
	for _, conflict := range conflicts {
		detail := fmt.Sprintf("organization conflict: %s; destination: %s", conflict.Reason, conflict.Destination)
		if conflict.OtherSource != "" && conflict.OtherSource != conflict.Source {
			detail += "; also used by: " + conflict.OtherSource
		}
		rows = append(rows, issueRow{file: conflict.Source, reason: detail, code: "organization-conflict"})
	}
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].file != rows[j].file {
			return rows[i].file < rows[j].file
		}
		return rows[i].code < rows[j].code
	})
	fmt.Print("\nSkipped files and organization conflicts:\n\n")

	report := reportexport.Report{Kind: csvIssuesKind, Headers: []string{"Skipped file", "Reason", "Reason_Code"}}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Skipped file", "Reason"})
	i := 0
	for _, row := range rows {
		report.Rows = append(report.Rows, []any{row.file, row.reason, row.code})
		t.AppendRow([]interface{}{i, row.file, row.reason})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(rows)})
	t.Render()

	c.exportReport(csvOutput, report)
}

func (c *Console) processMissingUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, settingsObj *settings.AppSettings, csvOutput string) {
	dateFormatter, err := dateformat.New(settingsObj.GUI.DateFormat)
	if err != nil {
		c.reportExportError(fmt.Errorf("gui.date_format: %w", err))
		return
	}
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.MissingContent.IgnoreUpdateIDs {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}

	incompleteTitles := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds, settingsObj.MissingContent.IgnoreDLCUpdates)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound available updates:\n\n")
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
		return
	}

	report := reportexport.Report{Kind: csvMissingUpdatesKind, Headers: []string{"Title", "TitleId", "Local version", "Latest Version", "Update Date"}}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Local version", "Latest Version", "Update Date"})
	i := 0
	for _, v := range incompleteTitles {
		date := dateFormatter.FormatISO(v.LatestUpdateDate)
		report.Rows = append(report.Rows, []any{v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, date})
		if date == "" {
			date = "Unknown"
		}

		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, date})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()

	c.exportReport(csvOutput, report)
}

func (c *Console) processMissingDLC(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, settingsObj *settings.AppSettings, csvOutput string) {
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.MissingContent.IgnoreDLCTitleIDs {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	incompleteTitles := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds)
	rows := flattenMissingDLC(incompleteTitles)
	if len(rows) != 0 {
		fmt.Print("\nFound missing DLCs:\n\n")
	} else {
		fmt.Print("\nYou have all the DLCS!\n\n")
		return
	}

	report := reportexport.Report{Kind: csvMissingDLCKind, Headers: []string{"Title", "TitleId", "Dlc (titleId - Name)"}}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Missing DLC", "DLC TitleId", "Title", "TitleId"})
	i := 0
	for _, row := range rows {
		values := missingDLCCSVRow(row)
		report.Rows = append(report.Rows, []any{values[0], values[1], values[2]})
		t.AppendRow([]interface{}{i, row.DLC.Name, row.DLC.Id, row.Game.Name, row.Game.Id})
		i++
	}
	t.AppendFooter(table.Row{"", "Total", len(rows), "", ""})
	t.Render()

	c.exportReport(csvOutput, report)
}

func (c *Console) prepareExportFolder(path, format string) string {
	if path == "" {
		return ""
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		c.reportExportError(fmt.Errorf("create %s export folder %q: %w", format, path, err))
		return ""
	}
	return path
}

func (c *Console) exportReport(csvOutput string, report reportexport.Report) {
	csv := CreateCsvFile(csvOutput, report.Headers)
	for _, row := range report.Rows {
		values := make([]string, len(row))
		for i, value := range row {
			if value != nil {
				values[i] = fmt.Sprint(value)
			}
		}
		csv.Write(values)
	}
	csv.Close()
	if c.xlsxOutput != "" {
		path := filepath.Join(c.xlsxOutput, reportexport.Filename(report.Kind, c.exportDate))
		if err := reportexport.WriteXLSX(path, report); err != nil {
			c.reportExportError(fmt.Errorf("save Excel export %q: %w", path, err))
		}
	}
}

func (c *Console) reportExportError(err error) {
	fmt.Printf("Export failed: %v\n", err)
	if c.sugarLogger != nil {
		c.sugarLogger.Errorf("Export failed: %v", err)
	}
}

type missingDLCRow struct {
	Game db.TitleAttributes
	DLC  process.MissingDLC
}

func missingDLCCSVRow(row missingDLCRow) []string {
	return []string{row.Game.Name, row.Game.Id, fmt.Sprintf("%s [%s]", row.DLC.Name, row.DLC.Id)}
}

func flattenMissingDLC(titles map[string]process.IncompleteTitle) []missingDLCRow {
	rows := make([]missingDLCRow, 0)
	for _, title := range titles {
		for _, dlc := range title.MissingDLC {
			rows = append(rows, missingDLCRow{Game: title.Attributes, DLC: dlc})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Game.Name != rows[j].Game.Name {
			return rows[i].Game.Name < rows[j].Game.Name
		}
		if rows[i].DLC.Name != rows[j].DLC.Name {
			return rows[i].DLC.Name < rows[j].DLC.Name
		}
		if rows[i].DLC.Id != rows[j].DLC.Id {
			return rows[i].DLC.Id < rows[j].DLC.Id
		}
		return rows[i].Game.Id < rows[j].Game.Id
	})
	return rows
}

func (c *Console) UpdateProgress(curr int, total int, message string) {
	progressBar.ChangeMax(total)
	progressBar.Set(curr)
}

type CsvFile struct {
	Writer *csv.Writer
	File   *os.File
}

func CreateCsvFile(output string, header []string) *CsvFile {
	if output == "" {
		return nil
	}

	file, err := os.Create(output)
	if err != nil {
		reportCsvExportError(output, "create", err)
		return nil
	}

	bom := []byte{0xEF, 0xBB, 0xBF}
	n, err := file.Write(bom)
	if err == nil && n != len(bom) {
		err = io.ErrShortWrite
	}
	if err != nil {
		reportCsvExportError(output, "write UTF-8 BOM to", err)
		if closeErr := file.Close(); closeErr != nil {
			reportCsvExportError(output, "close after failed initialization of", closeErr)
		}
		return nil
	}

	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		reportCsvExportError(output, "write CSV header to", err)
		if closeErr := file.Close(); closeErr != nil {
			reportCsvExportError(output, "close after failed initialization of", closeErr)
		}
		return nil
	}

	return &CsvFile{Writer: writer, File: file}
}

func reportCsvExportError(output, operation string, err error) {
	message := fmt.Sprintf("Failed to %s CSV export %q: %v", operation, output, err)
	fmt.Println(message)
	zap.S().Error(message)
}

func (csv *CsvFile) Close() {
	if csv != nil {
		csv.Writer.Flush()
		if err := csv.Writer.Error(); err != nil {
			reportCsvExportError(csv.File.Name(), "write", err)
		}
		if err := csv.File.Close(); err != nil {
			reportCsvExportError(csv.File.Name(), "close", err)
		}
	}
}

func (csv *CsvFile) Write(row []string) {
	if csv != nil {
		if err := csv.Writer.Write(row); err != nil {
			reportCsvExportError(csv.File.Name(), "write", err)
		}
	}
}
