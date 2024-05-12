package main

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/oneclickvirt/diskTest/disktest"
)

func main() {
	go func() {
		http.Get("https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2FdiskTest&count_bg=%2379C83D&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false")
	}()
	fmt.Println("项目地址:", "https://github.com/oneclickvirt/diskTest")
	// go run main.go -l en -d multi
	// go run main.go -l en -d single BUG
	languagePtr := flag.String("l", "", "Language parameter (en or zh)")
	multiDiskPtr := flag.String("d", "", "Enable multi disk check parameter (single or multi, default is single)")
	flag.Parse()
	var language, res string
	var isMultiCheck bool
	if *languagePtr == "" {
		language = "zh"
	} else {
		language = *languagePtr
	}
	if *multiDiskPtr == "" || *multiDiskPtr == "single" {
		isMultiCheck = false
	} else if *multiDiskPtr == "multi" {
		isMultiCheck = true
	}
	language = strings.ToLower(language)
	if runtime.GOOS == "windows" {
		res = disktest.WinsatTest(language, isMultiCheck) // BUG
	} else {
		res = disktest.FioTest(language, isMultiCheck)
		if res == "" {
			res = disktest.DDTest(language, isMultiCheck)
		}
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf(res)
	fmt.Println("--------------------------------------------------")
	// TODO https://github.com/devlights/diskio
}
