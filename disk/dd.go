package disk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/oneclickvirt/dd"
	. "github.com/oneclickvirt/defaultset"
	"github.com/shirou/gopsutil/disk"
)

// DDTest 通过 dd 命令测试硬盘IO
func DDTest(language string, enableMultiCheck bool, testPath string) string {
	var (
		result      string
		devices     []string
		mountPoints []string
	)
	parts, err := disk.Partitions(false)
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始DD测试硬盘IO")
		for _, part := range parts {
			Logger.Info("分区路径: " + part.Mountpoint + ", 设备: " + part.Device)
		}
	}
	if err == nil {
		for _, f := range parts {
			if !strings.Contains(f.Device, "vda") && !strings.Contains(f.Device, "snap") && !strings.Contains(f.Device, "loop") {
				if isWritableMountpoint(f.Mountpoint) {
					devices = append(devices, f.Device)
					mountPoints = append(mountPoints, f.Mountpoint)
					loggerInsert(Logger, "添加可写分区: "+f.Mountpoint+", 设备: "+f.Device)
				}
			}
		}
	}
	if language == "en" {
		result += "Test Path     Block Size         Direct Write(IOPS)                Direct Read(IOPS)\n"
	} else {
		result += "测试路径      块大小             直接写入(IOPS)                    直接读取(IOPS)\n"
	}
	var targetPath string
	if testPath == "" {
		if enableMultiCheck {
			targetPath = ""
		} else {
			if isWritableMountpoint("/root") {
				targetPath = "/root"
			} else {
				targetPath = "/tmp"
			}
		}
	} else {
		targetPath = testPath
	}
	blockNames := []string{"100MB-4K Block", "1GB-1M Block"}
	blockCounts := []string{"25600", "1000"}
	blockSizes := []string{"4k", "1M"}
	blockFiles := []string{"100MB.test", "1GB.test"}
	if targetPath != "" {
		blockNames, blockCounts, blockFiles = adjustDDTestSize(targetPath, blockSizes, blockNames, blockCounts, blockFiles)
	}
	for ind, bs := range blockSizes {
		loggerInsert(Logger, "开始测试块大小: "+bs+", 文件: "+blockFiles[ind])
		if testPath == "" {
			if enableMultiCheck {
				loggerInsert(Logger, "开始多路径测试")
				for index, path := range mountPoints {
					loggerInsert(Logger, "测试路径: "+path+", 设备: "+devices[index])
					adjustedBlockNames, adjustedBlockCounts, adjustedBlockFiles := adjustDDTestSize(path, []string{bs}, []string{blockNames[ind]}, []string{blockCounts[ind]}, []string{blockFiles[ind]})
					result += ddTest1(path, devices[index], adjustedBlockFiles[0], adjustedBlockNames[0], adjustedBlockCounts[0], bs)
				}
			} else {
				loggerInsert(Logger, "开始单路径测试(/root或/tmp)")
				result += ddTest2(blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
			}
		} else {
			loggerInsert(Logger, "测试指定路径: "+testPath)
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
		return "", err
	}
	if ddCmd == "" {
		return "", fmt.Errorf("execDDTest: ddCmd is NULL")
	}
	parts := strings.Split(ddCmd, " ")
	args := append(parts[1:], "if="+ifKey, "of="+ofKey, "bs="+bs, "count="+blockCount)
	if !strings.Contains(strings.ToLower(ddCmd), "darwin") {
		args = append(args, "oflag=direct")
	}
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
	tempText = string(outputBytes)
	loggerInsert(Logger, "DD测试原始输出: "+tempText)
	return tempText, nil
}

// ddTest1 无重试机制
func ddTest1(path, deviceName, blockFile, blockName, blockCount, bs string) string {
	var result string
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	// 写入测试
	tempText, err := execDDTest("/dev/zero", path+blockFile, bs, blockCount)
	defer os.Remove(path + blockFile)
	if err != nil {
		loggerInsert(Logger, "Write test error: "+err.Error())
	} else {
		result += fmt.Sprintf("%-10s", strings.TrimSpace(deviceName)) + "    " + fmt.Sprintf("%-15s", blockName) + "    "
		parsedResult := parseResultDD(tempText, blockCount)
		loggerInsert(Logger, "写入测试结果解析: "+parsedResult)
		result += parsedResult
		time.Sleep(1 * time.Second)
	}
	// 清理缓存, 避免影响测试结果
	syncCmd := exec.Command("sync")
	err = syncCmd.Run()
	if err != nil {
		loggerInsert(Logger, "sync command failed: "+err.Error())
	}
	// 读取测试
	tempText, err = execDDTest(path+blockFile, "/dev/null", bs, blockCount)
	defer os.Remove(path + blockFile)
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
		tempText, err = execDDTest(path+blockFile, path+"/read"+blockFile, bs, blockCount)
		defer os.Remove(path + "/read" + blockFile)
		if err != nil {
			loggerInsert(Logger, "Read test (second attempt) error: "+err.Error())
		}
	}
	parsedResult := parseResultDD(tempText, blockCount)
	loggerInsert(Logger, "读取测试结果解析: "+parsedResult)
	result += parsedResult
	result += "\n"
	return result
}

// ddTest2 有重试机制，重试至于 /tmp 目录
func ddTest2(blockFile, blockName, blockCount, bs string) string {
	var result string
	var testFilePath string
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	// 写入测试
	tempText, err := execDDTest("/dev/zero", "/root/"+blockFile, bs, blockCount)
	defer os.Remove("/root/" + blockFile)
	if err != nil {
		loggerInsert(Logger, "execDDTest error for /root/ path: "+err.Error())
	}
	if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
		strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
		loggerInsert(Logger, "写入测试到/root/失败，尝试写入到/tmp/: "+tempText)
		time.Sleep(1 * time.Second)
		tempText, err = execDDTest("/dev/zero", "/tmp/"+blockFile, bs, blockCount)
		defer os.Remove("/tmp/" + blockFile)
		if err != nil {
			loggerInsert(Logger, "execDDTest error for /tmp/ path: "+err.Error())
		}
		testFilePath = "/tmp/"
		result += fmt.Sprintf("%-10s", "/tmp") + "    " + fmt.Sprintf("%-15s", blockName) + "    "
	} else {
		testFilePath = "/root/"
		result += fmt.Sprintf("%-10s", "/root") + "    " + fmt.Sprintf("%-15s", blockName) + "    "
	}
	parsedResult := parseResultDD(tempText, blockCount)
	loggerInsert(Logger, "写入测试路径: "+testFilePath)
	loggerInsert(Logger, "写入测试结果解析: "+parsedResult)
	result += parsedResult
	// 清理缓存, 避免影响测试结果
	if testFilePath == "/tmp/" {
		syncCmd := exec.Command("sync")
		err = syncCmd.Run()
		if err != nil {
			loggerInsert(Logger, "sync command failed: "+err.Error())
		}
	}
	// 读取测试
	time.Sleep(1 * time.Second)
	tempText, err = execDDTest(testFilePath+blockFile, "/dev/null", bs, blockCount)
	defer os.Remove(testFilePath + blockFile)
	if err != nil {
		loggerInsert(Logger, "execDDTest read error for "+testFilePath+" path: "+err.Error())
	}
	// /dev/null 无法访问
	if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
		strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
		loggerInsert(Logger, "读取测试到/dev/null失败，尝试读取到/tmp/read文件: "+tempText)
		time.Sleep(1 * time.Second)
		tempText, err = execDDTest(testFilePath+blockFile, "/tmp/read"+blockFile, bs, blockCount)
		defer os.Remove("/tmp/read" + blockFile)
		if err != nil {
			loggerInsert(Logger, "execDDTest read error for /tmp/ path: "+err.Error())
		}
		// 如果/tmp/read也失败，尝试直接读取到当前目录
		if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") ||
			strings.Contains(tempText, "失败") || strings.Contains(tempText, "无效的参数") {
			loggerInsert(Logger, "读取测试到/tmp/read文件失败，尝试读取到当前目录: "+tempText)
			time.Sleep(1 * time.Second)
			// 使用原始文件名，但添加"read_"前缀，避免与源文件冲突
			tempText, err = execDDTest(testFilePath+blockFile, testFilePath+"read_"+blockFile, bs, blockCount)
			defer os.Remove(testFilePath + "read_" + blockFile)
			if err != nil {
				loggerInsert(Logger, "execDDTest read error for current path: "+err.Error())
			}
		}
	}
	parsedResult = parseResultDD(tempText, blockCount)
	loggerInsert(Logger, "读取测试结果解析: "+parsedResult)
	result += parsedResult
	result += "\n"
	return result
}
