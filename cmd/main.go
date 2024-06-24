package main

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/oneclickvirt/disktest/disktest"
)

func main() {
	go func() {
		http.Get("https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fdisktest&count_bg=%2379C83D&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/disktest")
	// go run main.go -l en -d multi
	// go run main.go -l en -d single -m fio
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")
	languagePtr := flag.String("l", "", "Language parameter (en or zh)")
	testMethodPtr := flag.String("m", "", "Specific Test Method (dd or fio)")
	multiDiskPtr := flag.String("d", "", "Enable multi disk check parameter (single or multi, default is single)")
	testPathPtr := flag.String("p", "", "Specific Test Disk Path (default is /root or C:)")
	flag.Parse()
	if showVersion {
		fmt.Println(disktest.DiskTestVersion)
		return
	}
	var language, res, testMethod, testPath string
	var isMultiCheck bool
	if *languagePtr == "" {
		language = "zh"
	} else {
		language = strings.ToLower(*languagePtr)
	}
	if *multiDiskPtr == "" || *multiDiskPtr == "single" {
		isMultiCheck = false
	} else if *multiDiskPtr == "multi" {
		isMultiCheck = true
	}
	if *testMethodPtr == "" || *testMethodPtr == "dd" {
		testMethod = "dd"
	} else if *testMethodPtr == "fio" {
		testMethod = "fio"
	}
	if *testPathPtr == "" {
		testPath = ""
	} else if *testPathPtr != "" {
		testPath = strings.TrimSpace(strings.ToLower(*testPathPtr))
	}
	if runtime.GOOS == "windows" {
		res = disktest.WinsatTest(language, isMultiCheck, testPath)
	} else {
		if testMethod == "fio" {
			res = disktest.FioTest(language, isMultiCheck, testPath)
		} else if testMethod == "dd" {
			res = disktest.DDTest(language, isMultiCheck, testPath)
		}
	}
	fmt.Printf(res)
	// TODO https://github.com/devlights/diskio
}
