package disktest

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// WinsatTest 通过windows自带系统工具测试IO
func WinsatTest(language string) string {
	var result string
	parts, err := disk.Partitions(true)
	if err == nil {
		if language == "zh" {
			result += "测试的硬盘                随机写入                  顺序读取                 顺序写入\n"
		} else {
			result += "Test Disk               Random Read             Sequential Read         Sequential Write\n"
		}
		for _, f := range parts {
			// winsat disk -drive 硬盘名字
			cmd := exec.Command("winsat", "disk", "-drive", f.Device)
			output, err := cmd.Output()
			if err == nil {
				result += fmt.Sprintf("%-20s", f.Device) + "    "
				tempList := strings.Split(string(output), "\n")
				for _, l := range tempList {
					if strings.Contains(l, "> Disk  Random 16.0 Read") {
						// 随机读取速度
						tempText := strings.TrimSpace(strings.ReplaceAll(l, "> Disk  Random 16.0 Read", ""))
						if tempText != "" {
							tpList := strings.Split(tempText, "MB/s")
							result += fmt.Sprintf("%-20s", strings.TrimSpace(tpList[0]+"MB/s["+strings.TrimSpace(tpList[len(tpList)-1])+"]")) + "    "
						}
					} else if strings.Contains(l, "> Disk  Sequential 64.0 Read") {
						// 顺序读取速度
						tempText := strings.TrimSpace(strings.ReplaceAll(l, "> Disk  Sequential 64.0 Read", ""))
						if tempText != "" {
							tpList := strings.Split(tempText, "MB/s")
							result += fmt.Sprintf("%-20s", strings.TrimSpace(tpList[0]+"MB/s["+strings.TrimSpace(tpList[len(tpList)-1])+"]")) + "    "
						}
					} else if strings.Contains(l, "> Disk  Sequential 64.0 Write") {
						// 顺序写入速度
						tempText := strings.TrimSpace(strings.ReplaceAll(l, "> Disk  Sequential 64.0 Write", ""))
						if tempText != "" {
							tpList := strings.Split(tempText, "MB/s")
							result += fmt.Sprintf("%-20s", strings.TrimSpace(tpList[0]+"MB/s["+strings.TrimSpace(tpList[len(tpList)-1])+"]")) + "    "
						}
					}
				}
			}
		}
	}
	return result
}

// DDTest 通过 dd 命令测试硬盘IO
func DDTest(language string) string {
	var result string
	parts, err := disk.Partitions(false)
	if err == nil {
		for _, f := range parts {
			fmt.Println(f)
			// lsblk -e 11 -n -o NAME | grep -v "vda" | grep -v "snap" | grep -v "loop"
		}
	}
	// https://github.com/spiritLHLS/ecs/blob/38f882433291384ec7e0aef8bd73349396139879/ecs.sh#L2056
	// dd if=/dev/zero of=/root/100MB.test bs=4k count=25600 oflag=direct
	//25600+0 records in
	//25600+0 records out
	//104857600 bytes (105 MB, 100 MiB) copied, 15.034 s, 7.0 MB/s
	// dd if=/dev/zero of=/root/1GB.test bs=1M count=1000 oflag=direct
	//1000+0 records in
	//1000+0 records out
	//1048576000 bytes (1.0 GB, 1000 MiB) copied, 2.7358 s, 383 MB/s
	// rm -rf 1GB.test 100MB.test
	return result
}
