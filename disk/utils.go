package disk

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"filepath"

	. "github.com/oneclickvirt/defaultset"
	"github.com/oneclickvirt/fio"
	"go.uber.org/zap"
)

// commandExists 检查命令是否存在
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// loggerInsert 插入日志
func loggerInsert(Logger *zap.Logger, st string) {
	if EnableLoger {
		Logger.Info(st)
	}
}

// isWritableMountpoint 检测挂载点是否为文件夹且可写入文件
func isWritableMountpoint(path string) bool {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	// 检测 mountpoint 是否是一个文件夹
	info, err := os.Stat(path)
	if err != nil {
		loggerInsert(Logger, "cannot stat path: "+err.Error())
		return false
	}
	if !info.IsDir() {
		loggerInsert(Logger, "path is not a directory: "+path)
		return false
	}
	// 尝试打开文件进行写入
	file, err := os.OpenFile(path+"/.temp_write_check", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		loggerInsert(Logger, "cannot open file for writing: "+err.Error())
		return false
	}
	defer file.Close()
	// 删除临时文件
	err = os.Remove(path + "/.temp_write_check")
	if err != nil {
		loggerInsert(Logger, "cannot remove temporary file: "+err.Error())
	}
	return true
}

// parseResultDD 提取dd测试的结果
func parseResultDD(tempText, blockCount string) string {
	var result string
	tp1 := strings.Split(tempText, "\n")
	var records, usageTime float64
	records, _ = strconv.ParseFloat(blockCount, 64)
	for _, t := range tp1 {
		// 检查FreeBSD格式: "1048576000 bytes transferred in 0.197523 secs (5308633827 bytes/sec)"
		if strings.Contains(t, "bytes transferred in") && strings.Contains(t, "bytes/sec") {
			parts := strings.Split(t, "transferred in")
			if len(parts) == 2 {
				timeAndSpeed := strings.Split(parts[1], "(")
				if len(timeAndSpeed) == 2 {
					// 提取时间
					timeStr := strings.Split(strings.TrimSpace(timeAndSpeed[0]), " ")[0]
					usageTime, _ = strconv.ParseFloat(timeStr, 64)
					// 提取速度
					speedStr := strings.Split(timeAndSpeed[1], " ")[0]
					speedFloat, _ := strconv.ParseFloat(speedStr, 64)
					// 转换为MB/s
					speedMBs := speedFloat / 1024 / 1024
					var speedUnit string
					if speedMBs >= 1024 {
						speedMBs = speedMBs / 1024
						speedUnit = "GB/s"
					} else {
						speedUnit = "MB/s"
					}
					// 计算IOPS
					iops := records / usageTime
					var iopsText string
					if iops >= 1000 {
						iopsText = strconv.FormatFloat(iops/1000, 'f', 2, 64) + "K IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
					} else {
						iopsText = strconv.FormatFloat(iops, 'f', 2, 64) + " IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
					}
					result += fmt.Sprintf("%-30s", fmt.Sprintf("%.2f %s(%s)", speedMBs, speedUnit, iopsText)) + "    "
				}
			}
		} else if strings.Contains(t, "bytes") || strings.Contains(t, "字节") {
			// Linux格式处理
			var tp2 []string
			if strings.Contains(t, "bytes") {
				tp2 = strings.Split(t, ",")
			} else {
				tp2 = strings.Split(t, "，")
			}
			// t 为 104857600 bytes (105 MB, 100 MiB) copied, 4.67162 s, 22.4 MB/s
			// t 为 104857600字节（105 MB，100 MiB）已复制，0.0569789 s，1.8 GB/s
			if len(tp2) == 4 {
				usageTime, _ = strconv.ParseFloat(strings.Split(strings.TrimSpace(tp2[2]), " ")[0], 64)
				ioSpeed := strings.Split(strings.TrimSpace(tp2[3]), " ")[0]
				ioSpeedFlat := strings.Split(strings.TrimSpace(tp2[3]), " ")[1]
				iops := records / usageTime
				var iopsText string
				if iops >= 1000 {
					iopsText = strconv.FormatFloat(iops/1000, 'f', 2, 64) + "K IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				} else {
					iopsText = strconv.FormatFloat(iops, 'f', 2, 64) + " IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				}
				result += fmt.Sprintf("%-30s", strings.TrimSpace(ioSpeed)+" "+ioSpeedFlat+"("+iopsText+")") + "    "
			}
			if len(tp2) == 3 {
				usageTime, _ = strconv.ParseFloat(strings.Split(strings.TrimSpace(tp2[1]), " ")[0], 64)
				ioSpeed := strings.Split(strings.TrimSpace(tp2[2]), " ")[0]
				ioSpeedFlat := strings.Split(strings.TrimSpace(tp2[2]), " ")[1]
				iops := records / usageTime
				var iopsText string
				if iops >= 1000 {
					iopsText = strconv.FormatFloat(iops/1000, 'f', 2, 64) + "K IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				} else {
					iopsText = strconv.FormatFloat(iops, 'f', 2, 64) + " IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
				}
				result += fmt.Sprintf("%-30s", strings.TrimSpace(ioSpeed)+" "+ioSpeedFlat+"("+iopsText+")") + "    "
			}
		}
	}
	return result
}

// formatIOPS 转换fio的测试中的IOPS的值
// rawType 支持 string 或 int
func formatIOPS(raw interface{}, rawType string) string {
	// 确保 raw 值不为空，如果为空则返回空字符串
	var iops int
	var err error
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ""
		}
		// 将 raw 字符串转换为整数
		iops, err = strconv.Atoi(v)
		if err != nil {
			return ""
		}
	case int:
		iops = v
	default:
		return ""
	}
	// 检查 IOPS 速度是否大于等于 10k
	if iops >= 10000 {
		// 将原始结果除以 1k
		result := float64(iops) / 1000.0
		// 将格式化后的结果保留一位小数（例如 x.x）
		resultStr := fmt.Sprintf("%.1fk", result)
		return resultStr
	}
	// 如果 IOPS 速度小于等于 1k，则返回原始值
	if rawType == "string" {
		return raw.(string)
	} else {
		return fmt.Sprintf("%d", iops)
	}
}

// formatSpeed 转换fio的测试中的TEST的值
// rawType 支持 string 或 float64
func formatSpeed(raw interface{}, _ string) string {
	var rawFloat float64
	var err error
	// 根据 type 确定如何处理 raw 的类型
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ""
		}
		// 将 raw 字符串转换为 float64
		rawFloat, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return ""
		}
	case float64:
		rawFloat = v
	default:
		return ""
	}
	// 初始化结果相关变量
	var resultFloat float64 = rawFloat
	var denom float64 = 1
	unit := "KB/s"
	// 根据速度大小确定单位
	if rawFloat >= 1000000 {
		denom = 1000000
		unit = "GB/s"
	} else if rawFloat >= 1000 {
		denom = 1000
		unit = "MB/s"
	}
	// 根据单位除以相应的分母以得到格式化后的结果
	resultFloat /= denom
	// 将格式化结果保留两位小数
	result := fmt.Sprintf("%.2f", resultFloat)
	// 将格式化结果值与单位拼接并返回结果
	return strings.Join([]string{result, unit}, " ")
}

// checkFioIOEngine 检查哪个IO引擎可用
func checkFioIOEngine() string {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	embeddedCmd, embeddedPath, err := fio.GetFIO()
	defer fio.CleanFio(embeddedPath)
	if err == nil {
		loggerInsert(Logger, "使用嵌入的fio二进制文件: "+embeddedPath)
	} else {
		loggerInsert(Logger, "无法获取嵌入的fio二进制文件: "+err.Error())
		return "psync"
	}
	if embeddedCmd == "" {
		return "psync"
	}
	parts := strings.Split(embeddedCmd, " ")
	var tempDir string
	var tempFile string
	if runtime.GOOS == "windows" {
		tempDir = os.Getenv("TEMP")
		if tempDir == "" {
			tempDir = os.Getenv("TMP")
		}
		if tempDir == "" {
			tempDir = "C:\\Windows\\Temp"
		}
		tempFile = filepath.Join(tempDir, "fio_engine_check")
	} else {
		tempDir = "/tmp"
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			tempDir = os.TempDir()
		}
		tempFile = filepath.Join(tempDir, "fio_engine_check")
	}
	engines := []string{}
	if runtime.GOOS == "linux" {
		engines = []string{"io_uring", "libaio", "posixaio"}
	} else if runtime.GOOS == "darwin" {
		engines = []string{"posixaio"}
	} else if runtime.GOOS == "windows" {
		engines = []string{"windowsaio"}
	} else {
		engines = []string{"posixaio"}
	}
	for _, engine := range engines {
		cmd := exec.Command(parts[0], append(parts[1:], "--name=check", "--ioengine="+engine, "--runtime=1", "--size=1M", "--direct=1", "--filename="+tempFile, "--minimal")...)
		_, err = cmd.CombinedOutput()
		defer func(filename string) {
			os.Remove(filename)
		}(tempFile)
		if err == nil {
			loggerInsert(Logger, engine+" IO引擎可用")
			return engine
		}
	}
	loggerInsert(Logger, "所有IO引擎都不可用，使用psync")
	return "psync"
}
