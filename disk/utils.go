package disk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
	. "github.com/oneclickvirt/defaultset"
	"github.com/oneclickvirt/fio"
	"go.uber.org/zap"
)

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func loggerInsert(Logger *zap.Logger, st string) {
	if EnableLoger {
		Logger.Info(st)
	}
}

func isWritableMountpoint(path string) bool {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	var testPath, tempFile string
	if runtime.GOOS == "windows" {
		if len(path) == 2 && path[1] == ':' {
			testPath = path + "\\"
		} else {
			testPath = path
		}
		info, err := os.Stat(testPath)
		if err != nil {
			loggerInsert(Logger, "cannot stat path: "+err.Error())
			return false
		}
		if !info.IsDir() {
			loggerInsert(Logger, "path is not a directory: "+testPath)
			return false
		}
		tempFile = filepath.Join(testPath, ".temp_write_check")
	} else {
		info, err := os.Stat(path)
		if err != nil {
			loggerInsert(Logger, "cannot stat path: "+err.Error())
			return false
		}
		if !info.IsDir() {
			loggerInsert(Logger, "path is not a directory: "+path)
			return false
		}
		tempFile = filepath.Join(path, ".temp_write_check")
	}
	file, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		loggerInsert(Logger, "cannot open file for writing: "+err.Error())
		return false
	}
	file.Close()
	err = os.Remove(tempFile)
	if err != nil {
		loggerInsert(Logger, "cannot remove temporary file: "+err.Error())
	}
	return true
}

func parseResultDD(tempText, blockCount string) string {
	var result string
	tp1 := strings.Split(tempText, "\n")
	var records, usageTime float64
	records, _ = strconv.ParseFloat(blockCount, 64)
	for _, t := range tp1 {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		// 检查FreeBSD格式: "1048576000 bytes transferred in 0.197523 secs (5308633827 bytes/sec)"
		if strings.Contains(t, "bytes transferred in") && strings.Contains(t, "bytes/sec") {
			parts := strings.Split(t, "transferred in")
			if len(parts) == 2 {
				timeAndSpeed := strings.Split(parts[1], "(")
				if len(timeAndSpeed) == 2 {
					timeStr := strings.Split(strings.TrimSpace(timeAndSpeed[0]), " ")[0]
					usageTime, _ = strconv.ParseFloat(timeStr, 64)
					speedStr := strings.Split(timeAndSpeed[1], " ")[0]
					speedFloat, _ := strconv.ParseFloat(speedStr, 64)
					speedMBs := speedFloat / 1024 / 1024
					var speedUnit string
					if speedMBs >= 1024 {
						speedMBs = speedMBs / 1024
						speedUnit = "GB/s"
					} else {
						speedUnit = "MB/s"
					}
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
			var tp2 []string
			if strings.Contains(t, "bytes") {
				tp2 = strings.Split(t, ",")
			} else {
				tp2 = strings.Split(t, "，")
			}
			// t 为 104857600 bytes (105 MB, 100 MiB) copied, 4.67162 s, 22.4 MB/s
			// t 为 104857600字节（105 MB，100 MiB）已复制，0.0569789 s，1.8 GB/s
			if len(tp2) >= 3 {
				var timeIndex, speedIndex int
				if len(tp2) == 4 {
					timeIndex = 2
					speedIndex = 3
				} else {
					timeIndex = 1
					speedIndex = 2
				}
				timeStr := strings.TrimSpace(tp2[timeIndex])
				speedStr := strings.TrimSpace(tp2[speedIndex])

				timeParts := strings.Split(timeStr, " ")
				if len(timeParts) >= 1 {
					usageTime, _ = strconv.ParseFloat(timeParts[0], 64)
				}
				speedParts := strings.Split(speedStr, " ")
				if len(speedParts) >= 2 {
					ioSpeed := speedParts[0]
					ioSpeedFlat := speedParts[1]
					iops := records / usageTime
					var iopsText string
					if iops >= 1000 {
						iopsText = strconv.FormatFloat(iops/1000, 'f', 2, 64) + "K IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
					} else {
						iopsText = strconv.FormatFloat(iops, 'f', 2, 64) + " IOPS, " + strconv.FormatFloat(usageTime, 'f', 2, 64) + "s"
					}
					result += fmt.Sprintf("%-30s", ioSpeed+" "+ioSpeedFlat+"("+iopsText+")") + "    "
				}
			}
		}
	}
	return result
}

// formatIOPS 转换fio的测试中的IOPS的值
// rawType 支持 string 或 int
func formatIOPS(raw interface{}, rawType string) string {
	var iops int
	var err error
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ""
		}
		iops, err = strconv.Atoi(v)
		if err != nil {
			return ""
		}
	case int:
		iops = v
	default:
		return ""
	}
	if iops >= 10000 {
		result := float64(iops) / 1000.0
		resultStr := fmt.Sprintf("%.1fk", result)
		return resultStr
	}
	if rawType == "string" {
		return raw.(string)
	} else {
		return fmt.Sprintf("%d", iops)
	}
}

func formatSpeed(raw interface{}, _ string) string {
	var rawFloat float64
	var err error
	switch v := raw.(type) {
	case string:
		if v == "" {
			return ""
		}
		rawFloat, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return ""
		}
	case float64:
		rawFloat = v
	default:
		return ""
	}
	var resultFloat float64 = rawFloat
	var denom float64 = 1
	unit := "KB/s"
	if rawFloat >= 1000000 {
		denom = 1000000
		unit = "GB/s"
	} else if rawFloat >= 1000 {
		denom = 1000
		unit = "MB/s"
	}
	resultFloat /= denom
	result := fmt.Sprintf("%.2f", resultFloat)
	return strings.Join([]string{result, unit}, " ")
}

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
	switch runtime.GOOS {
	case "linux":
		engines = []string{"io_uring", "libaio", "posixaio"}
	case "darwin":
		engines = []string{"posixaio"}
	case "windows":
		engines = []string{"windowsaio"}
	default:
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

// getMountPointColumnWidth 计算名称列的动态宽度
func getMountPointColumnWidth(name string) int {
	width := runewidth.StringWidth(name) // 按实际显示宽度计算
	if width < 5 {
		width = 5
	}
	return width
}

// getDefaultTestPaths 获取系统默认的测试路径
func getDefaultTestPaths() (string, string) {
	var rootPath, tmpPath string
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = "C:\\Users\\Default"
		}
		rootPath = userProfile
		tmpPath = os.TempDir()
	} else {
		rootPath = "/root"
		tmpPath = "/tmp"
	}
	return rootPath, tmpPath
}

// ensurePathExists 确保路径存在，如果不存在则创建
func ensurePathExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}
