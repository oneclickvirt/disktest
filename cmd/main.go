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
	languageSet, methodSet, multiDiskSet  bool
	pathSet, sizeSet, timeoutSet          bool
	runtimeSet                            bool
}

func parseCLI(args []string) (cliOptions, error) {
	opts := cliOptions{}
	fs := newFlagSet(&opts, io.Discard)
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	fs.Visit(func(current *flag.Flag) {
		switch current.Name {
		case "l":
			opts.languageSet = true
		case "m":
			opts.methodSet = true
		case "d":
			opts.multiDiskSet = true
		case "p":
			opts.pathSet = true
		case "duration":
			opts.runtimeSet = true
		case "timeout":
			opts.timeoutSet = true
		case "size":
			opts.sizeSet = true
		}
	})
	opts.language = strings.ToLower(strings.TrimSpace(opts.language))
	opts.testMethod = strings.ToLower(strings.TrimSpace(opts.testMethod))
	opts.multiDisk = strings.ToLower(strings.TrimSpace(opts.multiDisk))
	opts.path = strings.TrimSpace(opts.path)
	if opts.help || opts.version {
		return opts, nil
	}
	if opts.language != "" && opts.language != "en" && opts.language != "zh" {
		return opts, fmt.Errorf("language must be en or zh")
	}
	if opts.testMethod != "" && opts.testMethod != "fio" && opts.testMethod != "dd" && !(runtime.GOOS == "windows" && opts.testMethod == "winsat") {
		return opts, fmt.Errorf("disk method must be fio or dd")
	}
	if opts.multiDisk != "" && opts.multiDisk != "single" && opts.multiDisk != "multi" {
		return opts, fmt.Errorf("multi-disk mode must be single or multi")
	}
	if opts.pathSet && opts.path == "" {
		return opts, fmt.Errorf("disk path must not be empty when specified")
	}
	if opts.deep {
		opts.jsonOutput = true
	}
	if opts.jsonOutput {
		if opts.languageSet || opts.methodSet || opts.multiDiskSet {
			return opts, fmt.Errorf("-l, -m, and -d are not used with structured output")
		}
		if opts.runtimeSet && (opts.runtime <= 0 || opts.runtime > 10*time.Second) {
			return opts, fmt.Errorf("structured duration must be greater than zero and at most 10s")
		}
		maximum := 60 * time.Second
		if opts.deep {
			maximum = 3 * time.Minute
		}
		if opts.timeoutSet && (opts.timeout <= 0 || opts.timeout > maximum) {
			return opts, fmt.Errorf("structured timeout is outside the supported range")
		}
		if opts.sizeSet && (opts.sizeBytes < 16<<20 || opts.sizeBytes > 2<<30) {
			return opts, fmt.Errorf("structured size must be between 16 MiB and 2 GiB")
		}
	} else if opts.runtimeSet || opts.timeoutSet || opts.sizeSet {
		return opts, fmt.Errorf("-duration, -timeout, and -size require structured output")
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
		fmt.Fprintln(os.Stderr, sanitizeErrorText(err.Error()))
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
		if result.Status != "ok" {
			os.Exit(1)
		}
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
		testPath = strings.TrimSpace(testPath)
	}
	switch testMethod {
	case "fio":
		res = disk.FioTest(language, isMultiCheck, testPath)
		if res == "" {
			fallback := disk.DDTest(language, isMultiCheck, testPath)
			if fallback != "" {
				if language == "en" {
					res = "Fio test unavailable, switched to DD.\n"
				} else {
					res = "Fio测试不可用，已切换至DD测试。\n"
				}
				res += fallback
			} else if language == "en" {
				res = "Disk benchmark unavailable.\n"
			} else {
				res = "磁盘性能测试不可用。\n"
			}
		}
	case "dd":
		res = disk.DDTest(language, isMultiCheck, testPath)
		if res == "" {
			fallback := disk.FioTest(language, isMultiCheck, testPath)
			if fallback != "" {
				if language == "en" {
					res = "DD test unavailable, switched to Fio.\n"
				} else {
					res = "DD测试不可用，已切换至Fio测试。\n"
				}
				res += fallback
			} else if language == "en" {
				res = "Disk benchmark unavailable.\n"
			} else {
				res = "磁盘性能测试不可用。\n"
			}
		}
	default:
		if runtime.GOOS == "windows" {
			res = "Detected host is Windows, using Winsat for testing.\n"
			res += disk.WinsatTest(language, isMultiCheck, testPath)
		} else {
			res = "Unsupported test method specified.\n"
		}
	}
	fmt.Println(" --------------------------------------------------")
	fmt.Print(indentLegacyOutput(res))
	fmt.Println(" --------------------------------------------------")
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
	fmt.Println(" Repo:", "https://github.com/oneclickvirt/disktest")
}
