package disk

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/oneclickvirt/defaultset"
	"github.com/shirou/gopsutil/disk"
)

// WinsatTest 通过windows自带系统工具测试IO
func WinsatTest(language string, enableMultiCheck bool, testPath string) string {
	var result string
	parts, err := disk.Partitions(true)
	if err == nil {
		if language == "en" {
			result += "Test Disk               Random Read[Score]       Sequential Read[Score]  Sequential Write[Score]\n"
		} else {
			result += "测试的硬盘                随机写入[得分]            顺序读取[得分]            顺序写入[得分]\n"
		}
		if testPath == "" {
			if enableMultiCheck {
				for _, f := range parts {
					result += fmt.Sprintf("%-20s", f.Device) + "    "
					result += getDiskPerformance(f.Device)
				}
			} else {
				result += fmt.Sprintf("%-20s", "C:") + "    "
				result += getDiskPerformance("C:")
			}
		} else {
			result += fmt.Sprintf("%-20s", testPath) + "    "
			result += getDiskPerformance(testPath)
		}
	}
	return result
}

// getDiskPerformance 获取WIN的硬盘性能数据
func getDiskPerformance(device string) string {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	cmd := exec.Command("winsat", "disk", "-drive", device)
	output, err := cmd.Output()
	if err != nil {
		loggerInsert(Logger, "cannot match winsat command: "+err.Error())
		return ""
	}
	var result string
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
	result += "\n"
	return result
}
