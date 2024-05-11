package disktest

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// 获取硬盘性能数据
func getDiskPerformance(device string) string {
	cmd := exec.Command("winsat", "disk", "-drive", device)
	output, err := cmd.Output()
	if err != nil {
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
	return result
}

// isWritableMountpoint 检测挂载点是否为文件夹且可写入文件
func isWritableMountpoint(path string) bool {
	// 检测 mountpoint 是否是一个文件夹
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// 尝试打开文件进行写入
	file, err := os.OpenFile(path+"/.temp_write_check", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return false
	}
	defer file.Close()
	// 删除临时文件
	os.Remove(path + "/.temp_write_check")
	return true
}

// parseResultDD 提取dd测试的结果
func parseResultDD(tempText string) string {
	var result string
	tp1 := strings.Split(tempText, "\n")
	var records, usageTime float64
	records, _ = strconv.ParseFloat(strings.Split(strings.TrimSpace(tp1[0]), "+")[0], 64)
	for _, t := range tp1 {
		if strings.Contains(t, "bytes") {
			// t 为 104857600 bytes (105 MB, 100 MiB) copied, 4.67162 s, 22.4 MB/s
			tp2 := strings.Split(t, ",")
			if len(tp2) == 4 {
				usageTime, _ = strconv.ParseFloat(strings.Split(strings.TrimSpace(tp2[2]), " ")[0], 64)
				ioSpeed := strings.Split(strings.TrimSpace(tp2[3]), " ")[0]
				iops := records / usageTime
				var iopsText string
				if iops >= 1000 {
					iopsText = strconv.FormatFloat(iops/1000, 'f', 2, 64) + "K IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				} else {
					iopsText = strconv.FormatFloat(iops, 'f', 2, 64) + " IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				}
				result += fmt.Sprintf("%-30s", strings.TrimSpace(ioSpeed)+" MB/s("+iopsText+")") + "    "
			}
		}
	}
	return result
}
