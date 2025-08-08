package disk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	. "github.com/oneclickvirt/defaultset"
	"github.com/oneclickvirt/fio"
	"github.com/shirou/gopsutil/disk"
)

// generateFioTestHeader 生成FIO测试的表头
func generateFioTestHeader(language string, devices []string) string {
	// 计算所有设备名称中的最大宽度
	maxDeviceWidth := 10 // 默认最小宽度
	for _, device := range devices {
		deviceWidth := getDeviceColumnWidth(device)
		if deviceWidth > maxDeviceWidth {
			maxDeviceWidth = deviceWidth
		}
	}
	var header string
	if language == "en" {
		header = fmt.Sprintf("%-*s", maxDeviceWidth, "Test Path") + "    Block    Read(IOPS)              Write(IOPS)             Total(IOPS)\n"
	} else {
		header = fmt.Sprintf("%-*s", maxDeviceWidth, "测试路径") + "      块大小   读测试(IOPS)            写测试(IOPS)            总和(IOPS)\n"
	}
	return header
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

// FioTest 通过fio测试硬盘
func FioTest(language string, enableMultiCheck bool, testPath string) string {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始FIO测试硬盘")
	}
	var result string
	pathInfo, err := getTestPaths()
	if err != nil {
		if EnableLoger {
			Logger.Info("FioTest err: " + err.Error())
		}
		return ""
	}
	devices := pathInfo.Devices
	mountPoints := pathInfo.MountPoints
	result += generateFioTestHeader(language, devices)
	var defaultFioSize string
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		defaultFioSize = "512M"
	} else {
		defaultFioSize = "2G"
	}
	if testPath == "" {
		if enableMultiCheck {
			loggerInsert(Logger, "开始多路径FIO测试")
			for index, path := range mountPoints {
				loggerInsert(Logger, "测试路径: "+path+", 设备: "+devices[index])
				// 确保路径存在
				if err := ensurePathExists(path); err != nil {
					loggerInsert(Logger, "创建路径失败: "+path+", 错误: "+err.Error())
					continue
				}
				fioSize := adjustFioTestSize(path, defaultFioSize)
				loggerInsert(Logger, "FIO测试文件大小: "+fioSize)
				buildOutput, err := buildFioFile(path, fioSize)
				defer os.Remove(filepath.Join(path, "test.fio"))
				if err == nil {
					if buildOutput != "" {
						loggerInsert(Logger, "生成FIO测试文件输出: "+buildOutput)
					}
					time.Sleep(1 * time.Second)
					tempResult, err := execFioTest(path, strings.TrimSpace(devices[index]), fioSize)
					if err == nil {
						result += tempResult
					} else {
						loggerInsert(Logger, "执行FIO测试失败: "+err.Error())
					}
				} else {
					loggerInsert(Logger, "生成FIO测试文件失败: "+err.Error())
				}
			}
		} else {
			loggerInsert(Logger, "开始单路径FIO测试")
			var buildPath string
			var fioSize string
			rootPath, tmpPath := getDefaultTestPaths()
			if err := ensurePathExists(rootPath); err != nil {
				loggerInsert(Logger, "创建根路径失败: "+rootPath+", 错误: "+err.Error())
			}
			rootFioSize := adjustFioTestSize(rootPath, defaultFioSize)
			loggerInsert(Logger, rootPath+"路径FIO测试文件大小: "+rootFioSize)
			tempText, err := buildFioFile(rootPath, rootFioSize)
			defer os.Remove(filepath.Join(rootPath, "test.fio"))
			if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") || strings.Contains(tempText, "No such file or directory") {
				if EnableLoger {
					Logger.Info("在" + rootPath + "路径生成FIO测试文件失败，尝试" + tmpPath + "路径")
					if err != nil {
						Logger.Info("错误: " + err.Error())
					}
					if tempText != "" {
						Logger.Info("输出: " + tempText)
					}
				}
				if err := ensurePathExists(tmpPath); err != nil {
					loggerInsert(Logger, "创建临时路径失败: "+tmpPath+", 错误: "+err.Error())
				}
				tmpFioSize := adjustFioTestSize(tmpPath, defaultFioSize)
				loggerInsert(Logger, tmpPath+"路径FIO测试文件大小: "+tmpFioSize)
				buildOutput, err := buildFioFile(tmpPath, tmpFioSize)
				defer os.Remove(filepath.Join(tmpPath, "test.fio"))
				if err == nil {
					buildPath = tmpPath
					fioSize = tmpFioSize
					if EnableLoger && buildOutput != "" {
						Logger.Info("在" + tmpPath + "路径生成FIO测试文件输出: " + buildOutput)
					}
				} else if EnableLoger {
					Logger.Info("在" + tmpPath + "路径生成FIO测试文件失败: " + err.Error())
				}
			} else {
				buildPath = rootPath
				fioSize = rootFioSize
				if EnableLoger && tempText != "" {
					Logger.Info("在" + rootPath + "路径生成FIO测试文件输出: " + tempText)
				}
			}
			if buildPath != "" {
				loggerInsert(Logger, "使用路径进行FIO测试: "+buildPath)
				time.Sleep(1 * time.Second)
				tempResult, err := execFioTest(buildPath, buildPath, fioSize)
				if err == nil {
					result += tempResult
				} else if EnableLoger {
					Logger.Info("执行FIO测试失败: " + err.Error())
				}
			}
			// 检查是否有大于210GB的路径需要额外测试
			for index, path := range mountPoints {
				if path == rootPath || path == tmpPath {
					continue // 跳过已经测试过的默认路径
				}
				usage, err := disk.Usage(path)
				if err != nil {
					loggerInsert(Logger, "获取路径"+path+"磁盘使用情况失败: "+err.Error())
					continue
				}
				// 检查可用空间是否大于210GB (210 * 1024 * 1024 * 1024 bytes) (这是启用额外检测的条件)
				if usage.Free > uint64(210*1024*1024*1024) {
					loggerInsert(Logger, "检测到大容量路径: "+path+", 可用空间: "+fmt.Sprintf("%.2fGB", float64(usage.Free)/(1024*1024*1024))+", 进行额外FIO测试")
					// 确保路径存在
					if err := ensurePathExists(path); err != nil {
						loggerInsert(Logger, "创建大容量路径失败: "+path+", 错误: "+err.Error())
						continue
					}
					fioSize := adjustFioTestSize(path, defaultFioSize)
					loggerInsert(Logger, "大容量路径FIO测试文件大小: "+fioSize)
					buildOutput, err := buildFioFile(path, fioSize)
					defer os.Remove(filepath.Join(path, "test.fio"))
					if err == nil {
						if buildOutput != "" {
							loggerInsert(Logger, "生成大容量路径FIO测试文件输出: "+buildOutput)
						}
						time.Sleep(1 * time.Second)
						tempResult, err := execFioTest(path, strings.TrimSpace(devices[index]), fioSize)
						if err == nil {
							result += tempResult
						} else {
							loggerInsert(Logger, "执行大容量路径FIO测试失败: "+err.Error())
						}
					} else {
						loggerInsert(Logger, "生成大容量路径FIO测试文件失败: "+err.Error())
					}
				}
			}
		}
	} else {
		loggerInsert(Logger, "测试指定路径: "+testPath)
		if err := ensurePathExists(testPath); err != nil {
			loggerInsert(Logger, "创建指定路径失败: "+testPath+", 错误: "+err.Error())
			return "创建测试路径失败: " + err.Error()
		}
		fioSize := adjustFioTestSize(testPath, defaultFioSize)
		loggerInsert(Logger, "指定路径FIO测试文件大小: "+fioSize)
		tempText, err := buildFioFile(testPath, fioSize)
		defer os.Remove(filepath.Join(testPath, "test.fio"))
		if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") || strings.Contains(tempText, "No such file or directory") {
			if EnableLoger {
				Logger.Info("在指定路径生成FIO测试文件失败")
				if err != nil {
					Logger.Info("错误: " + err.Error())
				}
				if tempText != "" {
					Logger.Info("输出: " + tempText)
				}
			}
			return tempText
		}
		if EnableLoger && tempText != "" {
			Logger.Info("在指定路径生成FIO测试文件输出: " + tempText)
		}
		time.Sleep(1 * time.Second)
		tempResult, err := execFioTest(testPath, testPath, fioSize)
		if err == nil {
			result += tempResult
		} else if EnableLoger {
			Logger.Info("执行FIO测试失败: " + err.Error())
		}
	}
	return result
}

// adjustFioTestSize 根据可用磁盘空间调整FIO测试文件大小
func adjustFioTestSize(testPath, defaultSize string) string {
	usage, err := disk.Usage(testPath)
	if err != nil {
		loggerInsert(Logger, "获取磁盘使用情况失败: "+err.Error()+", 使用默认测试大小")
		return defaultSize
	}
	var requiredBytes uint64
	if defaultSize == "512M" {
		requiredBytes = 512 * 1024 * 1024
	} else { // defaultSize == "2G"
		requiredBytes = 2 * 1024 * 1024 * 1024
	}
	availableBytes := usage.Free
	if availableBytes < requiredBytes*3/2 {
		testSizeBytes := availableBytes / 5
		minSizeBytes := uint64(128 * 1024 * 1024)
		if testSizeBytes < minSizeBytes {
			testSizeBytes = minSizeBytes
		}
		maxSizeBytes := uint64(2 * 1024 * 1024 * 1024)
		if testSizeBytes > maxSizeBytes {
			testSizeBytes = maxSizeBytes
		}
		sizeStr := ""
		if defaultSize == "512M" {
			sizeMB := int(testSizeBytes / (1024 * 1024))
			sizeStr = fmt.Sprintf("%dM", sizeMB)
			loggerInsert(Logger, fmt.Sprintf("调整FIO测试大小从512M到%dM", sizeMB))
		} else {
			if testSizeBytes >= 1024*1024*1024 {
				sizeGB := float64(testSizeBytes) / (1024 * 1024 * 1024)
				sizeStr = fmt.Sprintf("%.1fG", sizeGB)
				loggerInsert(Logger, fmt.Sprintf("调整FIO测试大小从2G到%.1fG", sizeGB))
			} else {
				sizeMB := int(testSizeBytes / (1024 * 1024))
				sizeStr = fmt.Sprintf("%dM", sizeMB)
				loggerInsert(Logger, fmt.Sprintf("调整FIO测试大小从2G到%dM", sizeMB))
			}
		}
		return sizeStr
	}
	loggerInsert(Logger, fmt.Sprintf("可用空间充足(%d字节)，使用默认测试大小%s", availableBytes, defaultSize))
	return defaultSize
}

// buildFioFile 生成对应文件
func buildFioFile(path, fioSize string) (string, error) {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始生成FIO测试文件，路径: " + path + ", 大小: " + fioSize)
	}
	var args []string
	embeddedCmd, embeddedPath, err := fio.GetFIO()
	defer fio.CleanFio(embeddedPath)
	if err == nil {
		loggerInsert(Logger, "使用嵌入的fio二进制文件: "+embeddedPath)
	} else {
		loggerInsert(Logger, "fio不可用: "+err.Error())
	}
	args = strings.Split(embeddedCmd, " ")
	testFilePath := filepath.Join(path, "test.fio")
	args = append(args, "--name=setup", "--ioengine="+checkFioIOEngine(), "--rw=read", "--bs=64k", "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=1", "--gtod_reduce=1", "--filename="+testFilePath, "--direct=1", "--minimal")
	cmd1 := exec.Command(args[0], args[1:]...)
	stderr1, err := cmd1.StderrPipe()
	if err != nil {
		loggerInsert(Logger, "failed to get stderr pipe: "+err.Error())
		return "", err
	}
	if err := cmd1.Start(); err != nil {
		loggerInsert(Logger, "failed to start fio command: "+err.Error())
		return "", err
	}
	outputBytes, err := io.ReadAll(stderr1)
	if err != nil {
		loggerInsert(Logger, "failed to read stderr: "+err.Error())
		return "", err
	}
	tempText := string(outputBytes)
	if tempText != "" {
		loggerInsert(Logger, "生成FIO测试文件输出: "+tempText)
	}
	return tempText, nil
}

// execFioTest 使用fio测试文件进行测试
func execFioTest(path, devicename, fioSize string) (string, error) {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始执行FIO测试，路径: " + path + ", 设备: " + devicename + ", 大小: " + fioSize)
	}
	var result string
	var baseArgs []string
	// 获取可用的IO引擎
	ioEngine := checkFioIOEngine()
	loggerInsert(Logger, "使用IO引擎: "+ioEngine)
	embeddedCmd, embeddedPath, err := fio.GetFIO()
	defer fio.CleanFio(embeddedPath)
	if err == nil {
		loggerInsert(Logger, "使用嵌入的fio二进制文件: "+embeddedPath)
	} else {
		loggerInsert(Logger, "fio不可用: "+err.Error())
	}
	baseArgs = strings.Split(embeddedCmd, " ")
	testFilePath := filepath.Join(path, "test.fio")
	blockSizes := []string{"4k", "64k", "512k", "1m"}
	for _, BS := range blockSizes {
		loggerInsert(Logger, "开始测试块大小: "+BS)
		var args []string
		if commandExists("timeout") {
			args = append(args, "35")
		}
		var fioArgs []string
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			fioArgs = []string{
				"--name=rand_rw_" + BS,
				"--ioengine=" + ioEngine,
				"--rw=randrw",
				"--rwmixread=50",
				"--bs=" + BS,
				"--iodepth=64",
				"--numjobs=2",
				"--size=" + fioSize,
				"--runtime=30",
				"--direct=0",
				"--filename=" + testFilePath,
				"--group_reporting",
				"--minimal",
			}
		} else {
			fioArgs = []string{
				"--name=rand_rw_" + BS,
				"--ioengine=" + ioEngine,
				"--rw=randrw",
				"--rwmixread=50",
				"--bs=" + BS,
				"--iodepth=64",
				"--numjobs=2",
				"--size=" + fioSize,
				"--runtime=30",
				"--gtod_reduce=1",
				"--direct=1",
				"--filename=" + testFilePath,
				"--group_reporting",
				"--minimal",
			}
		}
		if commandExists("timeout") && runtime.GOOS != "windows" {
			cmd2 := exec.Command("timeout", append(args, append(baseArgs, fioArgs...)...)...)
			output, err := cmd2.Output()
			if err != nil {
				loggerInsert(Logger, "failed to execute fio command: "+err.Error())
				return "", err
			} else {
				tempText := string(output)
				result += processFioOutput(tempText, BS, devicename)
			}
		} else {
			cmd2 := exec.Command(baseArgs[0], append(baseArgs[1:], fioArgs...)...)
			output, err := cmd2.Output()
			if err != nil {
				loggerInsert(Logger, "failed to execute fio command: "+err.Error())
				return "", err
			} else {
				tempText := string(output)
				result += processFioOutput(tempText, BS, devicename)
			}
		}
	}
	return result, nil
}

// getDeviceColumnWidth 计算设备名称列的动态宽度
func getDeviceColumnWidth(devicename string) int {
	deviceWidth := len(devicename) + 2
	if deviceWidth < 10 {
		deviceWidth = 10
	}
	return deviceWidth
}

// processFioOutput 处理fio输出结果
func processFioOutput(tempText, BS, devicename string) string {
	var result string
	loggerInsert(Logger, "FIO测试原始输出("+BS+"): "+tempText)
	tempList := strings.Split(tempText, "\n")
	for _, l := range tempList {
		if strings.Contains(l, "rand_rw_"+BS) {
			tpList := strings.Split(l, ";")
			// IOPS
			DISK_IOPS_R := tpList[7]
			DISK_IOPS_W := tpList[48]
			DISK_IOPS_R_INT, _ := strconv.Atoi(DISK_IOPS_R)
			DISK_IOPS_W_INT, _ := strconv.Atoi(DISK_IOPS_W)
			DISK_IOPS := DISK_IOPS_R_INT + DISK_IOPS_W_INT
			// Speed
			DISK_TEST_R := tpList[6]
			DISK_TEST_W := tpList[47]
			DISK_TEST_R_INT, _ := strconv.ParseFloat(DISK_TEST_R, 64)
			DISK_TEST_W_INT, _ := strconv.ParseFloat(DISK_TEST_W, 64)
			DISK_TEST := DISK_TEST_R_INT + DISK_TEST_W_INT
			// 记录解析后的结果到日志
			loggerInsert(Logger, "块大小: "+BS+", 读取IOPS: "+DISK_IOPS_R+", 写入IOPS: "+DISK_IOPS_W+
				", 总IOPS: "+strconv.Itoa(DISK_IOPS)+", 读取速度: "+DISK_TEST_R+
				", 写入速度: "+DISK_TEST_W+", 总速度: "+fmt.Sprintf("%f", DISK_TEST))
			deviceWidth := getDeviceColumnWidth(devicename)
			// 拼接输出文本
			result += fmt.Sprintf("%-*s", deviceWidth, devicename) + "    "
			result += fmt.Sprintf("%-5s", BS) + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_R, "string")+"("+formatIOPS(DISK_IOPS_R, "string")+")") + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_W, "string")+"("+formatIOPS(DISK_IOPS_W, "string")+")") + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST, "float64")+"("+formatIOPS(DISK_IOPS, "int")+")") + "    "
			result += "\n"
		}
	}
	return result
}
