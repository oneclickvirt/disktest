package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/oneclickvirt/disktest/disk"
)

type cliOptions struct {
	help, version, jsonOutput, deep, log  bool
	language, testMethod, multiDisk, path string
	sizeBytes                             int64
	timeout, runtime                      time.Duration
}

func parseCLI(args []string) (cliOptions, error) {
	opts := cliOptions{}
	fs := newFlagSet(&opts, io.Discard)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if opts.runtime < 0 || opts.timeout < 0 || opts.sizeBytes < 0 {
		return opts, fmt.Errorf("duration, timeout, and size must not be negative")
	}
	if opts.deep {
		opts.jsonOutput = true
	}
	return opts, nil
}

func newFlagSet(opts *cliOptions, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("disktest", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.BoolVar(&opts.help, "h", false, "Show help information")
	fs.BoolVar(&opts.version, "v", false, "Show version")
	fs.StringVar(&opts.language, "l", "", "Language parameter (en or zh)")
	fs.StringVar(&opts.testMethod, "m", "", "Specific Test Method (dd or fio)")
	fs.StringVar(&opts.multiDisk, "d", "", "Enable multi disk check parameter (single or multi, default is single)")
	fs.StringVar(&opts.path, "p", "", "Specific Test Disk Path (default is /root or C:)")
	fs.BoolVar(&opts.log, "log", false, "Enable logging")
	fs.BoolVar(&opts.jsonOutput, "json", false, "Print the Go structured FIO result as JSON")
	fs.BoolVar(&opts.jsonOutput, "structured", false, "Print the Go structured FIO result as JSON")
	fs.BoolVar(&opts.deep, "deep", false, "Run the explicit deep FIO matrix")
	fs.DurationVar(&opts.runtime, "duration", 0, "Per-scenario FIO runtime (for example 5s)")
	fs.DurationVar(&opts.timeout, "timeout", 0, "FIO matrix timeout (for example 60s)")
	fs.Int64Var(&opts.sizeBytes, "size", 0, "Temporary test-file size in bytes")
	return fs
}

func printCLIHelp(program string) {
	fmt.Printf("Usage: %s [options]\n", program)
	newFlagSet(&cliOptions{}, os.Stdout).PrintDefaults()
}

func selectCLIAction(opts cliOptions) string {
	if opts.help {
		return "help"
	}
	if opts.version {
		return "version"
	}
	if opts.jsonOutput {
		return "structured"
	}
	return "legacy"
}

func main() {
	opts, err := parseCLI(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	disk.EnableLoger = opts.log
	action := selectCLIAction(opts)
	if action == "help" || action == "version" {
		printLegacyHeader()
		if action == "help" {
			printCLIHelp(os.Args[0])
			return
		}
		fmt.Println(disk.DiskTestVersion)
		return
	}
	if action == "structured" {
		if strings.TrimSpace(opts.testMethod) != "" || strings.TrimSpace(opts.multiDisk) != "" {
			fmt.Fprintln(os.Stderr, "-m/--test-method and -d/--multi-disk are only supported by legacy output")
			os.Exit(2)
		}
		config := disk.MatrixConfig{Path: opts.path, SizeBytes: opts.sizeBytes, Runtime: opts.runtime, MaxDuration: opts.timeout}
		ctx := context.Background()
		result := disk.MatrixResult{}
		if opts.deep {
			result = disk.RunDeepFioMatrix(ctx, config)
		} else {
			result = disk.RunStandardFioMatrix(ctx, config)
		}
		encoded, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, marshalErr)
			return
		}
		fmt.Println(string(encoded))
		return
	}
	printLegacyHeader()
	language, testMethod, testPath, multiDisk := opts.language, opts.testMethod, opts.path, opts.multiDisk
	var res string
	var isMultiCheck bool
	if language == "" {
		language = "zh"
	} else {
		language = strings.ToLower(language)
	}
	if multiDisk == "" || multiDisk == "single" {
		isMultiCheck = false
	} else if multiDisk == "multi" {
		isMultiCheck = true
	}
	if testMethod == "" || testMethod == "fio" {
		testMethod = "fio"
	} else if testMethod == "dd" {
		testMethod = "dd"
	}
	if testPath == "" {
		testPath = ""
	} else if testPath != "" {
		testPath = strings.TrimSpace(strings.ToLower(testPath))
	}
	switch testMethod {
	case "fio":
		res = disk.FioTest(language, isMultiCheck, testPath)
		if res == "" {
			res = "Fio test failed, switching to DD for testing.\n"
			res += disk.DDTest(language, isMultiCheck, testPath)
		}
	case "dd":
		res = disk.DDTest(language, isMultiCheck, testPath)
		if res == "" {
			res = "DD test failed, switching to Fio for testing.\n"
			res += disk.FioTest(language, isMultiCheck, testPath)
		}
	default:
		if runtime.GOOS == "windows" {
			res = "Detected host is Windows, using Winsat for testing.\n"
			res += disk.WinsatTest(language, isMultiCheck, testPath)
		} else {
			res = "Unsupported test method specified.\n"
		}
	}
	fmt.Println("--------------------------------------------------")
	fmt.Print(res)
	fmt.Println("--------------------------------------------------")
	// TODO https://github.com/devlights/diskio
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
}

func printLegacyHeader() {
	go func() {
		http.Get("https://hits.spiritlhl.net/disktest.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false")
	}()
	fmt.Println("Repo:", "https://github.com/oneclickvirt/disktest")
}
