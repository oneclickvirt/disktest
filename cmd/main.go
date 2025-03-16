package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/oneclickvirt/disktest/disk"
)

func main() {
	go func() {
		http.Get("https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fdisktest&count_bg=%2379C83D&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/disktest")
	var showVersion, help bool
	var language, testMethod, testPath, multiDisk string
	disktestFlag := flag.NewFlagSet("disktest", flag.ContinueOnError)
	disktestFlag.BoolVar(&help, "h", false, "Show help information")
	disktestFlag.BoolVar(&showVersion, "v", false, "Show version")
	disktestFlag.StringVar(&language, "l", "", "Language parameter (en or zh)")
	disktestFlag.StringVar(&testMethod, "m", "", "Specific Test Method (dd or fio)")
	disktestFlag.StringVar(&multiDisk, "d", "", "Enable multi disk check parameter (single or multi, default is single)")
	disktestFlag.StringVar(&testPath, "p", "", "Specific Test Disk Path (default is /root or C:)")
	disktestFlag.BoolVar(&disk.EnableLoger, "log", false, "Enable logging")
	disktestFlag.Parse(os.Args[1:])
	if help {
		fmt.Printf("Usage: %s [options]\n", os.Args[0])
		disktestFlag.PrintDefaults()
		return
	}
	if showVersion {
		fmt.Println(disk.DiskTestVersion)
		return
	}
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
	if runtime.GOOS == "windows" {
		if testMethod != "winsat" && testMethod != "" {
			res = "Detected host is Windows, using Winsat for testing.\n"
		}
		res = disk.WinsatTest(language, isMultiCheck, testPath)
	} else {
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
			res = "Unsupported test method specified.\n"
		}
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf(res)
	fmt.Println("--------------------------------------------------")
	// TODO https://github.com/devlights/diskio
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		fmt.Println("Press Enter to exit...")
		fmt.Scanln()
	}
}
