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

	"github.com/oneclickvirt/dd"
	. "github.com/oneclickvirt/defaultset"
	"github.com/shirou/gopsutil/disk"
)

// generateDDTestHeader 生成DD测试的表头
func generateDDTestHeader(language string, actualTestPaths []string) string {
	mountPointsWidth := 10 // 默认最小宽度
	for _, path := range actualTestPaths {
		pathWidth := getMountPointColumnWidth(path)
		if pathWidth > mountPointsWidth {
			mountPointsWidth = pathWidth
		}
	}
	mountPointsWidth += 2
	var header string
	if language == "en" {
		header = fmt.Sprintf("%-*s    %-15s    %-30s    %-30s\n",
			mountPointsWidth, "Test Path",
			"Block Size",
			"Direct Write(IOPS)",
			"Direct Read(IOPS)")
	} else {
		header = fmt.Sprintf("%-*s    %-15s    %-30s    %-30s\n",
			mountPointsWidth, "测试路径",
			"块大小",
			"直接写入(IOPS)",
			"直接读取(IOPS)")
	}
	return header
}

// DDTest 通过 dd 命令测试硬盘IO
func DDTest(language string, enableMultiCheck bool, testPath string) string {
	var result string
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始DD测试硬盘IO")
	}
	pathInfo, err := getTestPaths()
	if err != nil {
		if EnableLoger {
			Logger.Info("DDTest err: " + err.Error())
		}
		return ""
	}
	devices := pathInfo.Devices
	mountPoints := pathInfo.MountPoints
	var actualTestPaths []string
	var targetPath string
	if testPath == "" {
		if enableMultiCheck {
			targetPath = ""
			actualTestPaths = mountPoints
		} else {
			rootPath, tmpPath := getDefaultTestPaths()
			if runtime.GOOS == "darwin" {
				targetPath = tmpPath
				actualTestPaths = []string{tmpPath}
			} else if isWritableMountpoint(rootPath) {
				targetPath = rootPath
				actualTestPaths = []string{rootPath}
			} else {
				targetPath = tmpPath
				actualTestPaths = []string{tmpPath}
			}
			// 检查是否有大于210GB的路径需要额外测试
			for _, path := range mountPoints {
				if path == rootPath || path == tmpPath {
					continue
				}
				usage, err := disk.Usage(path)
				if err != nil {
					continue
				}
				if usage.Free > uint64(210*1024*1024*1024) {
					actualTestPaths = append(actualTestPaths, path)
				}
			}
		}
	} else {
		targetPath = testPath
		actualTestPaths = []string{testPath}
	}
	result += generateDDTestHeader(language, actualTestPaths)
	blockNames := []string{"100MB-4K Block", "1GB-1M Block"}
	blockCounts := []string{"25600", "1000"}
	blockSizes := []string{"4k", "1M"}
	blockFiles := []string{"100MB.test", "1GB.test"}
	if targetPath != "" {
		// 确保目标路径存在
		if err := ensurePathExists(targetPath); err != nil {
			loggerInsert(Logger, "创建目标路径失败: "+targetPath+", 错误: "+err.Error())
		}
		blockNames, blockCounts, blockFiles = adjustDDTestSize(targetPath, blockSizes, blockNames, blockCounts, blockFiles)
	}
	for ind, bs := range blockSizes {
		loggerInsert(Logger, "开始测试块大小: "+bs+", 文件: "+blockFiles[ind])
		if testPath == "" {
			if enableMultiCheck {
				loggerInsert(Logger, "开始多路径测试")
				for index, path := range mountPoints {
					loggerInsert(Logger, "测试路径: "+path+", 设备: "+devices[index])
					// 确保路径存在
					if err := ensurePathExists(path); err != nil {
						loggerInsert(Logger, "创建路径失败: "+path+", 错误: "+err.Error())
						continue
					}
					adjustedBlockNames, adjustedBlockCounts, adjustedBlockFiles := adjustDDTestSize(path, []string{bs}, []string{blockNames[ind]}, []string{blockCounts[ind]}, []string{blockFiles[ind]})
					result += ddTest1(path, devices[index], adjustedBlockFiles[0], adjustedBlockNames[0], adjustedBlockCounts[0], bs)
				}
			} else {
				rootPath, tmpPath := getDefaultTestPaths()
				loggerInsert(Logger, "开始单路径测试("+rootPath+"或"+tmpPath+")")
				result += ddTest2(blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
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
						loggerInsert(Logger, "检测到大容量路径: "+path+", 可用空间: "+fmt.Sprintf("%.2fGB", float64(usage.Free)/(1024*1024*1024))+", 进行额外测试")
						// 确保路径存在
						if err := ensurePathExists(path); err != nil {
							loggerInsert(Logger, "创建大容量路径失败: "+path+", 错误: "+err.Error())
							continue
						}
						adjustedBlockNames, adjustedBlockCounts, adjustedBlockFiles := adjustDDTestSize(path, []string{bs}, []string{blockNames[ind]}, []string{blockCounts[ind]}, []string{blockFiles[ind]})
						result += ddTest1(path, devices[index], adjustedBlockFiles[0], adjustedBlockNames[0], adjustedBlockCounts[0], bs)
					}
				}
			}
		} else {
			loggerInsert(Logger, "测试指定路径: "+testPath)
			// 确保指定路径存在
			if err := ensurePathExists(testPath); err != nil {
				loggerInsert(Logger, "创建指定路径失败: "+testPath+", 错误: "+err.Error())
				return "创建测试路径失败: " + err.Error()
			}
			result += ddTest1(testPath, testPath, blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
		}
	}
	return result
}

// adjustDDTestSize 根据可用磁盘空间调整DD测试参数
func adjustDDTestSize(testPath string, blockSizes, blockNames, blockCounts, blockFiles []string) ([]string, []string, []string) {
	adjustedBlockNames := make([]string, len(blockNames))
	adjustedBlockCounts := make([]string, len(blockCounts))
	adjustedBlockFiles := make([]string, len(blockFiles))
	copy(adjustedBlockNames, blockNames)
	copy(adjustedBlockCounts, blockCounts)
	copy(adjustedBlockFiles, blockFiles)
	usage, err := disk.Usage(testPath)
	if err != nil {
		loggerInsert(Logger, "获取磁盘使用情况失败: "+err.Error()+", 使用默认测试参数")
		return blockNames, blockCounts, blockFiles
	}
	availableBytes := usage.Free
	for i, bs := range blockSizes {
		var requiredBytes uint64
		if bs == "4k" {
			requiredBytes = 100 * 1024 * 1024
		} else { // bs == "1M"
			requiredBytes = 1024 * 1024 * 1024
		}
		if availableBytes < requiredBytes*3/2 {
			testSizeBytes := availableBytes / 5
			minSizeBytes := uint64(20 * 1024 * 1024)
			if testSizeBytes < minSizeBytes {
				testSizeBytes = minSizeBytes
			}
			if bs == "4k" {
				sizeMB := int(testSizeBytes / (1024 * 1024))
				if sizeMB > 50 {
					sizeMB = 50
				}
				adjustedBlockFiles[i] = fmt.Sprintf("%dMB.test", sizeMB)
				adjustedBlockNames[i] = fmt.Sprintf("%dMB-4K Block", sizeMB)
				adjustedBlockCounts[i] = fmt.Sprintf("%d", sizeMB*256)
				loggerInsert(Logger, fmt.Sprintf("调整4K块测试大小为%dMB, 块数%s", sizeMB, adjustedBlockCounts[i]))
			} else {
				sizeMB := int(testSizeBytes / (1024 * 1024))
				if sizeMB > 500 {
					sizeMB = 500
				}
				if sizeMB >= 1024 {
					adjustedBlockFiles[i] = fmt.Sprintf("%dGB.test", sizeMB/1024)
					adjustedBlockNames[i] = fmt.Sprintf("%dGB-1M Block", sizeMB/1024)
				} else {
					adjustedBlockFiles[i] = fmt.Sprintf("%dMB.test", sizeMB)
					adjustedBlockNames[i] = fmt.Sprintf("%dMB-1M Block", sizeMB)
				}
				adjustedBlockCounts[i] = fmt.Sprintf("%d", sizeMB)
				loggerInsert(Logger, fmt.Sprintf("调整1M块测试大小为%dMB, 块数%s", sizeMB, adjustedBlockCounts[i]))
			}
		} else {
			loggerInsert(Logger, fmt.Sprintf("可用空间充足(%d字节)，使用默认测试参数", availableBytes))
		}
	}
	return adjustedBlockNames, adjustedBlockCounts, adjustedBlockFiles
}

// getDevNullPath 获取系统对应的null设备路径
func getDevNullPath() string {
	if runtime.GOOS == "windows" {
		return "NUL"
	}
	return "/dev/null"
}

// getDevZeroPath 获取系统对应的zero设备路径
func getDevZeroPath() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return "/dev/zero"
}

// execDDTest 执行dd命令测试硬盘IO，并回传结果和测试错误
func execDDTest(ifKey, ofKey, bs, blockCount string) (string, error) {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	var tempText string
	ddCmd, ddPath, err := dd.GetDD()
	defer dd.CleanDD(ddPath)
	if err != nil {
		loggerInsert(Logger, "获取DD命令失败: "+err.Error())
		return "", err
	}
	if ddCmd == "" {
		loggerInsert(Logger, "DD命令为空")
		return "", fmt.Errorf("execDDTest: ddCmd is NULL")
	}
	loggerInsert(Logger, fmt.Sprintf("执行DD命令: %s, if=%s, of=%s, bs=%s, count=%s", ddCmd, ifKey, ofKey, bs, blockCount))
	parts := strings.Split(ddCmd, " ")
	args := append(parts[1:], "if="+ifKey, "of="+ofKey, "bs="+bs, "count="+blockCount)
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		args = append(args, "oflag=direct")
	}
	loggerInsert(Logger, fmt.Sprintf("完整命令参数: %s %s", parts[0], strings.Join(args, " ")))
	cmd2 := exec.Command(parts[0], args...)
	stderr2, err := cmd2.StderrPipe()
	if err != nil {
		loggerInsert(Logger, "failed to get StderrPipe: "+err.Error())
		return "", err
	}
	if err := cmd2.Start(); err != nil {
		loggerInsert(Logger, "failed to start command: "+err.Error())
		return "", err
	}
	outputBytes, err := io.ReadAll(stderr2)
	if err != nil {
		loggerInsert(Logger, "failed to read stderr: "+err.Error())
		return "", err
	}
	// 等待命令完成并检查退出状态
	if err := cmd2.Wait(); err != nil {
		loggerInsert(Logger, "DD命令执行失败: "+err.Error())
		tempText = string(outputBytes)
		loggerInsert(Logger, "DD命令错误输出: "+tempText)
		return tempText, err
	}
	tempText = string(outputBytes)
	loggerInsert(Logger, "DD测试原始输出: "+tempText)
	// 检查输出是否为空
	if strings.TrimSpace(tempText) == "" {
		loggerInsert(Logger, "DD测试输出为空，可能是Windows系统下的正常现象")
		// 在Windows下，dd可能不输出到stderr，我们检查文件是否创建成功
		if runtime.GOOS == "windows" && strings.Contains(ofKey, ".test") {
			if info, err := os.Stat(ofKey); err == nil && info.Size() > 0 {
				loggerInsert(Logger, fmt.Sprintf("文件创建成功，大小: %d 字节", info.Size()))
				// 为Windows生成模拟输出，用于解析
				tempText = fmt.Sprintf("%s bytes transferred", blockCount)
			} else {
				loggerInsert(Logger, "文件创建失败或大小为0")
			}
		}
	}
	return tempText, nil
}

// ddTest1 无重试机制
func ddTest1(path, deviceName, blockFile, blockName, blockCount, bs string) string {
	var result string
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	fullBlockFile := filepath.Join(path, blockFile)
	// 写入测试
	var writeSource string
	if runtime.GOOS == "windows" {
		zeroFile := filepath.Join(path, "zero_temp")
		defer os.Remove(zeroFile)
		if err := createZeroFile(zeroFile, bs, blockCount); err != nil {
			loggerInsert(Logger, "创建零文件失败: "+err.Error())
			return ""
		}
		writeSource = zeroFile
	} else {
		writeSource = getDevZeroPath()
	}
	tempText, err := execDDTest(writeSource, fullBlockFile, bs, blockCount)
	defer os.Remove(fullBlockFile)
	// 动态计算第一列宽度
	deviceWidth := getMountPointColumnWidth(strings.TrimSpace(deviceName))
	result += fmt.Sprintf("%-*s    %-15s    ", deviceWidth, strings.TrimSpace(deviceName), blockName)
	if err != nil {
		loggerInsert(Logger, "Write test error: "+err.Error())
		result += fmt.Sprintf("%-30s    ", "写入失败")
	} else {
		parsedResult := parseResultDD(tempText, blockCount)
		loggerInsert(Logger, "写入测试结果解析: "+parsedResult)
		if strings.TrimSpace(parsedResult) == "" {
			parsedResult = "无法解析结果"
		}
		result += fmt.Sprintf("%-30s    ", parsedResult)
		time.Sleep(1 * time.Second)
	}
	// 同步
	syncCmd := exec.Command("sync")
	err = syncCmd.Run()
	if err != nil && runtime.GOOS != "windows" {
		loggerInsert(Logger, "sync command failed: "+err.Error())
	}
	// 读取测试
	devNull := getDevNullPath()
	tempText, err = execDDTest(fullBlockFile, devNull, bs, blockCount)
	defer os.Remove(fullBlockFile)
	if err != nil {
		loggerInsert(Logger, "Read test error: "+err.Error())
	}
	if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
		strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
		if err != nil {
			loggerInsert(Logger, "Read test (first attempt) error: "+err.Error())
			loggerInsert(Logger, "Read test (first attempt) output: "+tempText)
		}
		time.Sleep(1 * time.Second)
		readTestFile := filepath.Join(path, "read_"+blockFile)
		tempText, err = execDDTest(fullBlockFile, readTestFile, bs, blockCount)
		defer os.Remove(readTestFile)
		if err != nil {
			loggerInsert(Logger, "Read test (second attempt) error: "+err.Error())
		}
	}
	if err != nil {
		result += fmt.Sprintf("%-30s", "读取失败")
	} else {
		parsedResult := parseResultDD(tempText, blockCount)
		loggerInsert(Logger, "读取测试结果解析: "+parsedResult)
		if strings.TrimSpace(parsedResult) == "" {
			parsedResult = "无法解析结果"
		}
		result += fmt.Sprintf("%-30s", parsedResult)
	}
	result += "\n"
	return result
}

// ddTest2 有重试机制，重试至临时目录
func ddTest2(blockFile, blockName, blockCount, bs string) string {
	var result string
	var testFilePath string
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	rootPath, tmpPath := getDefaultTestPaths()
	if runtime.GOOS == "darwin" {
		testFilePath = tmpPath
		deviceWidth := getMountPointColumnWidth(tmpPath)
		result += fmt.Sprintf("%-*s    %-15s    ", deviceWidth, tmpPath, blockName)
		fullBlockFile := filepath.Join(tmpPath, blockFile)
		writeSource := getDevZeroPath()
		tempText, err := execDDTest(writeSource, fullBlockFile, bs, blockCount)
		defer os.Remove(fullBlockFile)
		if err != nil {
			loggerInsert(Logger, "execDDTest error for "+tmpPath+" path: "+err.Error())
			result += fmt.Sprintf("%-30s    ", "写入失败")
		} else {
			parsedResult := parseResultDD(tempText, blockCount)
			loggerInsert(Logger, "写入测试路径: "+testFilePath)
			loggerInsert(Logger, "写入测试结果解析: "+parsedResult)
			if strings.TrimSpace(parsedResult) == "" {
				parsedResult = "无法解析结果"
			}
			result += fmt.Sprintf("%-30s    ", parsedResult)
		}
	} else {
		var writeSource string
		if runtime.GOOS == "windows" {
			zeroFile := filepath.Join(rootPath, "zero_temp")
			defer os.Remove(zeroFile)
			if err := createZeroFile(zeroFile, bs, blockCount); err != nil {
				loggerInsert(Logger, "创建零文件失败: "+err.Error())
				zeroFile = filepath.Join(tmpPath, "zero_temp")
				defer os.Remove(zeroFile)
				if err := createZeroFile(zeroFile, bs, blockCount); err != nil {
					loggerInsert(Logger, "在临时目录创建零文件失败: "+err.Error())
					return ""
				}
			}
			writeSource = zeroFile
		} else {
			writeSource = getDevZeroPath()
		}
		fullBlockFile := filepath.Join(rootPath, blockFile)
		tempText, err := execDDTest(writeSource, fullBlockFile, bs, blockCount)
		defer os.Remove(fullBlockFile)
		if err != nil {
			loggerInsert(Logger, "execDDTest error for "+rootPath+" path: "+err.Error())
		}
		if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
			strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") || err != nil {
			loggerInsert(Logger, "写入测试到"+rootPath+"失败，尝试写入到"+tmpPath+": "+tempText)
			time.Sleep(1 * time.Second)
			if runtime.GOOS == "windows" {
				zeroFile := filepath.Join(tmpPath, "zero_temp")
				defer os.Remove(zeroFile)
				if err := createZeroFile(zeroFile, bs, blockCount); err != nil {
					loggerInsert(Logger, "在临时目录创建零文件失败: "+err.Error())
					return ""
				}
				writeSource = zeroFile
			}
			fullBlockFile = filepath.Join(tmpPath, blockFile)
			tempText, err = execDDTest(writeSource, fullBlockFile, bs, blockCount)
			defer os.Remove(fullBlockFile)
			if err != nil {
				loggerInsert(Logger, "execDDTest error for "+tmpPath+" path: "+err.Error())
			}
			testFilePath = tmpPath
			deviceWidth := getMountPointColumnWidth(tmpPath)
			result += fmt.Sprintf("%-*s    %-15s    ", deviceWidth, tmpPath, blockName)
		} else {
			testFilePath = rootPath
			deviceWidth := getMountPointColumnWidth(rootPath)
			result += fmt.Sprintf("%-*s    %-15s    ", deviceWidth, rootPath, blockName)
		}
		if err != nil {
			result += fmt.Sprintf("%-30s    ", "写入失败")
		} else {
			parsedResult := parseResultDD(tempText, blockCount)
			loggerInsert(Logger, "写入测试路径: "+testFilePath)
			loggerInsert(Logger, "写入测试结果解析: "+parsedResult)
			if strings.TrimSpace(parsedResult) == "" {
				parsedResult = "无法解析结果"
			}
			result += fmt.Sprintf("%-30s    ", parsedResult)
		}
	}
	if runtime.GOOS != "windows" {
		syncCmd := exec.Command("sync")
		err := syncCmd.Run()
		if err != nil {
			loggerInsert(Logger, "sync command failed: "+err.Error())
		}
	}
	time.Sleep(1 * time.Second)
	// 读取测试
	fullBlockFile := filepath.Join(testFilePath, blockFile)
	devNull := getDevNullPath()
	tempText, err := execDDTest(fullBlockFile, devNull, bs, blockCount)
	defer os.Remove(fullBlockFile)
	if err != nil {
		loggerInsert(Logger, "execDDTest read error for "+testFilePath+" path: "+err.Error())
	}
	if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
		strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
		loggerInsert(Logger, "读取测试到null设备失败，尝试读取到临时文件: "+tempText)
		time.Sleep(1 * time.Second)
		readFile := filepath.Join(tmpPath, "read_"+blockFile)
		tempText, err = execDDTest(fullBlockFile, readFile, bs, blockCount)
		defer os.Remove(readFile)
		if err != nil {
			loggerInsert(Logger, "execDDTest read error for tmp path: "+err.Error())
		}
		if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
			strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
			loggerInsert(Logger, "读取测试到临时文件失败，尝试读取到当前目录: "+tempText)
			time.Sleep(1 * time.Second)
			readFile = filepath.Join(testFilePath, "read_"+blockFile)
			tempText, err = execDDTest(fullBlockFile, readFile, bs, blockCount)
			defer os.Remove(readFile)
			if err != nil {
				loggerInsert(Logger, "execDDTest read error for current path: "+err.Error())
			}
		}
	}
	if err != nil {
		result += fmt.Sprintf("%-30s", "读取失败")
	} else {
		parsedResult := parseResultDD(tempText, blockCount)
		loggerInsert(Logger, "读取测试结果解析: "+parsedResult)
		if strings.TrimSpace(parsedResult) == "" {
			parsedResult = "无法解析结果"
		}
		result += fmt.Sprintf("%-30s", parsedResult)
	}
	result += "\n"
	return result
}

// createZeroFile 为Windows系统创建指定大小的零文件
func createZeroFile(filePath, bs, blockCount string) error {
	var blockSize int64
	if strings.HasSuffix(strings.ToLower(bs), "k") {
		size := strings.TrimSuffix(strings.ToLower(bs), "k")
		if s, err := strconv.ParseInt(size, 10, 64); err == nil {
			blockSize = s * 1024
		} else {
			return fmt.Errorf("invalid block size: %s", bs)
		}
	} else if strings.HasSuffix(strings.ToUpper(bs), "M") {
		size := strings.TrimSuffix(strings.ToUpper(bs), "M")
		if s, err := strconv.ParseInt(size, 10, 64); err == nil {
			blockSize = s * 1024 * 1024
		} else {
			return fmt.Errorf("invalid block size: %s", bs)
		}
	} else {
		if s, err := strconv.ParseInt(bs, 10, 64); err == nil {
			blockSize = s
		} else {
			return fmt.Errorf("invalid block size: %s", bs)
		}
	}
	count, err := strconv.ParseInt(blockCount, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid block count: %s", blockCount)
	}
	totalSize := blockSize * count
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 8192) // 8KB 缓冲区
	written := int64(0)
	for written < totalSize {
		writeSize := int64(len(buffer))
		if totalSize-written < writeSize {
			writeSize = totalSize - written
		}
		n, err := file.Write(buffer[:writeSize])
		if err != nil {
			return err
		}
		written += int64(n)
	}
	return nil
}
