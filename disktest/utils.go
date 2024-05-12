package disktest

import (
	"fmt"
	"io"
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

// formatIOPS 转换fio的测试中的IOPS的值
// rawType 支持 string 或 int
func formatIOPS(raw interface{}, rawType string) string {
	// Ensure raw value is not empty, if it is, return blank
	var iops int
	var err error
	if rawType == "string" {
		if raw.(string) == "" {
			return ""
		}
		// Convert raw string to integer
		iops, err = strconv.Atoi(raw.(string))
		if err != nil {
			return ""
		}
	} else if rawType == "int" {
		iops = raw.(int)
	} else {
		return ""
	}
	// Check if IOPS speed > 1k
	if iops >= 1000 {
		// Divide the raw result by 1k
		result := float64(iops) / 1000.0
		// Shorten the formatted result to one decimal place (i.e. x.x)
		resultStr := fmt.Sprintf("%.1fk", result)
		return resultStr
	}
	// If IOPS speed <= 1k, return the original value
	return raw.(string)
}

// formatSpeed 转换fio的测试中的TEST的值
// rawType 支持 string 或 float64
func formatSpeed(raw interface{}, rawType string) string {
	var rawFloat float64
	var err error
	if rawType == "string" {
		if raw.(string) == "" {
			return ""
		}
		// disk speed in KB/s
		rawFloat, err = strconv.ParseFloat(raw.(string), 64)
		if err != nil {
			return ""
		}
	} else if rawType == "float64" {
		rawFloat = raw.(float64)
	} else {
		return ""
	}
	var resultFloat float64 = rawFloat
	var denom float64 = 1
	unit := "KB/s"
	// check if disk speed >= 1 GB/s
	if rawFloat >= 1000000 {
		denom = 1000000
		unit = "GB/s"
	} else if rawFloat >= 1000 { // check if disk speed < 1 GB/s && >= 1 MB/s
		denom = 1000
		unit = "MB/s"
	}
	// divide the raw result to get the corresponding formatted result (based on determined unit)
	resultFloat /= denom
	// shorten the formatted result to two decimal places (i.e. x.xx)
	result := fmt.Sprintf("%.2f", resultFloat)
	// concat formatted result value with units and return result
	return strings.Join([]string{result, unit}, " ")
}

// execDDTest 执行dd命令测试硬盘IO，并回传结果和测试错误
func execDDTest(ifKey, ofKey, bs, blockCount string) (string, error) {
	var tempText string
	cmd2 := exec.Command("sudo", "dd", "if="+ifKey, "of="+ofKey, "bs="+bs, "count="+blockCount, "oflag=direct")
	stderr2, err := cmd2.StderrPipe()
	if err == nil {
		if err := cmd2.Start(); err == nil {
			outputBytes, err := io.ReadAll(stderr2)
			if err == nil {
				tempText = string(outputBytes)
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	} else {
		return "", err
	}
	return tempText, nil
}
