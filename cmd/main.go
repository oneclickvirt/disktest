package main

import (
	"fmt"

	"github.com/oneclickvirt/diskTest/disktest"
)

func main() {
	// res := disktest.WinsatTest("zh", false)
	// res := disktest.DDTest("en", true)
	res := disktest.FioTest("zh", false)
	fmt.Println(res)
	// fio test
	// https://github.com/devlights/diskio
}
