package console

import (
	"flag"
	"strconv"

	"go.uber.org/zap"
)

type flagValue struct {
	set   bool
	value string
}

func (sf *flagValue) Set(x string) error {
	sf.value = x
	sf.set = true
	return nil
}
func (sf *flagValue) String() string {
	return sf.value
}
func (sf *flagValue) Bool() bool {
	convert, _ := strconv.ParseBool(sf.value)
	return convert
}
func (sf *flagValue) IsSet() bool {
	return sf.set
}

type ConsoleFlags struct {
	Mode       flagValue
	NspFolder  flagValue
	Recursive  flagValue
	ExportCsv  flagValue
	ExportXlsx flagValue
}

var mode string
var nspFolder string
var recursive bool
var exportCsv string
var exportXlsx string

func InitializeFlags() {
	if flag.Parsed() {
		return
	}

	flag.StringVar(&mode, "m", "", "console or gui, overrides the gui flag in settings.json")
	flag.StringVar(&nspFolder, "f", "", "path to NSP folder")
	flag.BoolVar(&recursive, "r", true, "recursively scan sub folders")
	flag.StringVar(&exportCsv, "e", "", "output missing updates, dlcs and issues as csv")
	flag.StringVar(&exportXlsx, "export-xlsx", "", "output missing updates, dlcs and issues as Excel workbooks")

	flag.Parse()
}

var (
	consoleFlagsInstance *ConsoleFlags
)

func GetFlagsValues() *ConsoleFlags {
	if consoleFlagsInstance != nil {
		return consoleFlagsInstance
	}
	if !flag.Parsed() {
		InitializeFlags()
	}

	flagset := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagset[f.Name] = true })

	modeFlag := &flagValue{}
	if flagset["m"] {
		modeFlag.Set(mode)
	}

	nspFolderFlag := &flagValue{}
	if flagset["f"] {
		nspFolderFlag.Set(nspFolder)
	}

	recursiveFlag := &flagValue{}
	if flagset["r"] {
		recursiveFlag.Set(strconv.FormatBool(recursive))
	}

	exportCsvFlag := &flagValue{}
	if flagset["e"] {
		exportCsvFlag.Set(exportCsv)
	}
	exportXlsxFlag := &flagValue{}
	if flagset["export-xlsx"] {
		exportXlsxFlag.Set(exportXlsx)
	}

	consoleFlagsInstance = &ConsoleFlags{
		Mode:       *modeFlag,
		NspFolder:  *nspFolderFlag,
		Recursive:  *recursiveFlag,
		ExportCsv:  *exportCsvFlag,
		ExportXlsx: *exportXlsxFlag,
	}

	return consoleFlagsInstance
}

func LogFlags(sugar *zap.SugaredLogger) {
	values := GetFlagsValues()

	logFlag(sugar, "m", values.Mode)
	logFlag(sugar, "f", values.NspFolder)
	logFlag(sugar, "r", values.Recursive)
	logFlag(sugar, "e", values.ExportCsv)
	logFlag(sugar, "export-xlsx", values.ExportXlsx)
}

func logFlag(sugar *zap.SugaredLogger, flagName string, flag flagValue) {
	if !flag.set {
		sugar.Infof("[Flag -%v: %v]", flagName, "<missing>")
	} else {
		sugar.Infof("[Flag -%v: %v]", flagName, flag.value)
	}
}
