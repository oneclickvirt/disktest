package main

import (
	"fmt"

	"github.com/oneclickvirt/diskTest/disktest"
)

func main() {
	// https://github.com/devlights/diskio
	// res := disktest.WinsatTest("zh", false)
	res := disktest.DDTest("zh", false)
	fmt.Println(res)
	// fio test
	// https://github.com/masonr/yet-another-bench-script/blob/0ad4c4e85694dbcf0958d8045c2399dbd0f9298c/yabs.sh#L435
	// fio --name=setup --ioengine=libaio --rw=read --bs=64k --iodepth=64 --numjobs=2 --size=$FIO_SIZE --runtime=1 --gtod_reduce=1 --filename="$DISK_PATH/test.fio" --direct=1 --minimal
}
