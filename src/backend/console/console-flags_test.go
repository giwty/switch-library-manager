package console

import (
	"flag"
	"os"
	"testing"
)

func TestExportFlagsCanBeCombined(t *testing.T) {
	originalFlags, originalArgs, originalInstance := flag.CommandLine, os.Args, consoleFlagsInstance
	originalMode, originalFolder, originalRecursive, originalCSV, originalXLSX := mode, nspFolder, recursive, exportCsv, exportXlsx
	t.Cleanup(func() {
		flag.CommandLine, os.Args, consoleFlagsInstance = originalFlags, originalArgs, originalInstance
		mode, nspFolder, recursive, exportCsv, exportXlsx = originalMode, originalFolder, originalRecursive, originalCSV, originalXLSX
	})
	for _, test := range []struct {
		name      string
		args      []string
		csv, xlsx bool
	}{
		{"disabled", nil, false, false},
		{"csv", []string{"-e", "csv reports"}, true, false},
		{"xlsx", []string{"-export-xlsx", "excel reports"}, false, true},
		{"both", []string{"-e", "csv reports", "-export-xlsx", "excel reports"}, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("slm", flag.ContinueOnError)
			os.Args = append([]string{"slm"}, test.args...)
			consoleFlagsInstance = nil
			InitializeFlags()
			got := GetFlagsValues()
			if got.ExportCsv.IsSet() != test.csv || got.ExportXlsx.IsSet() != test.xlsx {
				t.Fatalf("flags = %+v", got)
			}
			if test.csv && got.ExportCsv.String() != "csv reports" {
				t.Error("CSV folder missing")
			}
			if test.xlsx && got.ExportXlsx.String() != "excel reports" {
				t.Error("Excel folder missing")
			}
		})
	}
}
