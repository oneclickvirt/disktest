package disktest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// WinsatTest 通过windows自带系统工具测试IO
func WinsatTest(language string, enableMultiCheck bool, testPath string) string {
	var result string
	parts, err := disk.Partitions(true)
	if err == nil {
		if language == "en" {
			result += "Test Disk               Random Read             Sequential Read         Sequential Write\n"
		} else {
			result += "测试的硬盘                随机写入                  顺序读取                 顺序写入\n"
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

// ddTest1 无重试机制
func ddTest1(path, deviceName, blockFile, blockName, blockCount, bs string) string {
	var result string
	// 写入测试
	// dd if=/dev/zero of=/tmp/100MB.test bs=4k count=25600 oflag=direct
	tempText, err := execDDTest("/dev/zero", path+blockFile, bs, blockCount)
	defer os.Remove(path + blockFile)
	if err == nil {
		result += fmt.Sprintf("%-10s", strings.TrimSpace(deviceName)) + "    " + fmt.Sprintf("%-15s", blockName) + "    "
		result += parseResultDD(tempText)
	}
	// 读取测试
	// dd if=/tmp/100MB.test of=/dev/null bs=4k count=25600 oflag=direct
	tempText, err = execDDTest(path+blockFile, "/dev/null", bs, blockCount)
	defer os.Remove(path + blockFile)
	if err != nil || strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
		tempText, _ = execDDTest(path+blockFile, path+"/read"+blockFile, bs, blockCount)
		defer os.Remove(path + "/read" + blockFile)
	}
	result += parseResultDD(tempText)
	result += "\n"
	return result
}

// ddTest2 有重试机制，重试至于 /tmp 目录
func ddTest2(blockFile, blockName, blockCount, bs string) string {
	var result string
	// 写入测试
	var testFilePath string
	tempText, err := execDDTest("/dev/zero", "/root/"+blockFile, bs, blockCount)
	defer os.Remove("/root/" + blockFile)
	if err != nil || strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
		tempText, _ = execDDTest("/dev/zero", "/tmp/"+blockFile, bs, blockCount)
		defer os.Remove("/tmp/" + blockFile)
		testFilePath = "/tmp/"
		result += fmt.Sprintf("%-10s", "/tmp") + "    " + fmt.Sprintf("%-15s", blockName) + "    "
	} else {
		testFilePath = "/root/"
		result += fmt.Sprintf("%-10s", "/root") + "    " + fmt.Sprintf("%-15s", blockName) + "    "
	}
	result += parseResultDD(tempText)
	// 读取测试
	tempText, err = execDDTest("/root/"+blockFile, "/dev/null", bs, blockCount)
	defer os.Remove("/root/" + blockFile)
	if err != nil || strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
		tempText, _ = execDDTest(testFilePath+blockFile, "/tmp/read"+blockFile, bs, blockCount)
		defer os.Remove(testFilePath + blockFile)
		defer os.Remove("/tmp/read" + blockFile)
	}
	result += parseResultDD(tempText)
	result += "\n"
	return result
}

// DDTest 通过 dd 命令测试硬盘IO
func DDTest(language string, enableMultiCheck bool, testPath string) string {
	var (
		result      string
		devices     []string
		mountPoints []string
	)
	parts, err := disk.Partitions(false)
	if err == nil {
		for _, f := range parts {
			if !strings.Contains(f.Device, "vda") && !strings.Contains(f.Device, "snap") && !strings.Contains(f.Device, "loop") {
				if isWritableMountpoint(f.Mountpoint) {
					devices = append(devices, f.Device)
					mountPoints = append(mountPoints, f.Mountpoint)
				}
			}
		}
	}
	if language == "en" {
		result += "Test Path     Block Size         Direct Write                      Direct Read\n"
	} else {
		result += "测试路径      块大小             直接写入                          直接读取\n"
	}
	blockNames := []string{"100MB-4K Block", "1GB-1M Block"}
	blockCounts := []string{"25600", "1000"}
	blockSizes := []string{"4k", "1M"}
	blockFiles := []string{"100MB.test", "1GB.test"}
	for ind, bs := range blockSizes {
		if testPath == "" {
			if enableMultiCheck {
				for index, path := range mountPoints {
					result += ddTest1(path, devices[index], blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
				}
			} else {
				result += ddTest2(blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
			}
		} else {
			result += ddTest1(testPath, testPath, blockFiles[ind], blockNames[ind], blockCounts[ind], bs)
		}
	}
	return result
}

// buildFioFile 生成对应文件
func buildFioFile(path, fioSize string) (string, error) {
	// https://github.com/masonr/yet-another-bench-script/blob/0ad4c4e85694dbcf0958d8045c2399dbd0f9298c/yabs.sh#L435
	// fio --name=setup --ioengine=libaio --rw=read --bs=64k --iodepth=64 --numjobs=2 --size=512MB --runtime=1 --gtod_reduce=1 --filename="/tmp/test.fio" --direct=1 --minimal
	var tempText string
	cmd1 := exec.Command("sudo", "fio", "--name=setup", "--ioengine=libaio", "--rw=read", "--bs=64k", "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=1", "--gtod_reduce=1",
		"--filename=\""+path+"/test.fio\"", "--direct=1", "--minimal")
	stderr1, err := cmd1.StderrPipe()
	if err == nil {
		if err := cmd1.Start(); err == nil {
			outputBytes, err := io.ReadAll(stderr1)
			if err == nil {
				tempText = string(outputBytes)
				return tempText, nil
			} else {
				return "", err
			}
		} else {
			return "", err
		}
	} else {
		return "", err
	}
}

// FioTest 通过fio测试硬盘
func FioTest(language string, enableMultiCheck bool, testPath string) string {
	var (
		result, fioSize string
		devices         []string
		mountPoints     []string
	)
	parts, err := disk.Partitions(false)
	if err == nil {
		for _, f := range parts {
			if !strings.Contains(f.Device, "vda") && !strings.Contains(f.Device, "snap") && !strings.Contains(f.Device, "loop") {
				if isWritableMountpoint(f.Mountpoint) {
					devices = append(devices, f.Device)
					mountPoints = append(mountPoints, f.Mountpoint)
				}
			}
		}
	}
	// fio --version
	cmd := exec.Command("fio", "--version")
	output, _ := cmd.Output()
	if strings.Contains(string(output), "failed") {
		return ""
	}
	if language == "en" {
		result += "Test Path     Block    Read(IOPS)              Write(IOPS)             Total(IOPS)\n"
	} else {
		result += "测试路径      块大小   读测试(IOPS)            写测试(IOPS)            总和(IOPS)\n"
	}
	// 生成测试文件
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "x86" || runtime.GOARCH == "x86_64" {
		fioSize = "2G"
	} else {
		fioSize = "512MB"
	}
	if testPath == "" {
		if enableMultiCheck {
			for index, path := range mountPoints {
				_, err := buildFioFile(path, fioSize)
				defer os.Remove(path + "/test.fio")
				if err == nil {
					// 测试
					blockSizes := []string{"4k", "64k", "512k", "1m"}
					for _, BS := range blockSizes {
						// timeout 35 fio --name=rand_rw_4k --ioengine=libaio --rw=randrw --rwmixread=50 --bs=4k --iodepth=64 --numjobs=2 --size=512MB --runtime=30 --gtod_reduce=1 --direct=1 --filename="/tmp/test.fio" --group_reporting --minimal
						cmd2 := exec.Command("timeout", "35", "sudo", "fio", "--name=rand_rw_"+BS, "--ioengine=libaio", "--rw=randrw", "--rwmixread=50", "--bs="+BS, "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=30", "--gtod_reduce=1", "--direct=1", "--filename=\""+path+"/test.fio\"", "--group_reporting", "--minimal")
						output, err := cmd2.Output()
						if err == nil {
							tempText := string(output)
							tempList := strings.Split(tempText, "\n")
							for _, l := range tempList {
								if strings.Contains(l, "rand_rw_"+BS) {
									tpList := strings.Split(l, ";")
									// IOPS
									DISK_IOPS_R := tpList[8]
									DISK_IOPS_W := tpList[49]
									DISK_IOPS_R_INT, _ := strconv.Atoi(DISK_IOPS_R)
									DISK_IOPS_W_INT, _ := strconv.Atoi(DISK_IOPS_W)
									DISK_IOPS := DISK_IOPS_R_INT + DISK_IOPS_W_INT
									// Speed
									DISK_TEST_R := tpList[7]
									DISK_TEST_W := tpList[48]
									DISK_TEST_R_INT, _ := strconv.ParseFloat(DISK_TEST_R, 64)
									DISK_TEST_W_INT, _ := strconv.ParseFloat(DISK_TEST_W, 64)
									DISK_TEST := DISK_TEST_R_INT + DISK_TEST_W_INT
									// 拼接输出文本
									result += fmt.Sprintf("%-10s", strings.TrimSpace(devices[index])) + "    "
									result += fmt.Sprintf("%-5s", BS) + "    "
									result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_R, "string")+"("+formatIOPS(DISK_IOPS_R, "string")+")") + "    "
									result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_W, "string")+"("+formatIOPS(DISK_IOPS_W, "string")+")") + "    "
									result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST, "float64")+"("+formatIOPS(DISK_IOPS, "int")+")") + "    "
									result += "\n"
								}
							}
						}
					}
				}
			}
		} else {
			var buildPath string
			tempText, err := buildFioFile("/root", fioSize)
			defer os.Remove("/root/test.fio")
			if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") {
				_, err = buildFioFile("/tmp", fioSize)
				if err == nil {
					buildPath = "/tmp"
				} else {
					buildPath = ""
				}
			} else {
				buildPath = "/root"
			}
			if buildPath != "" {
				// 测试
				blockSizes := []string{"4k", "64k", "512k", "1m"}
				for _, BS := range blockSizes {
					// timeout 35 fio --name=rand_rw_4k --ioengine=libaio --rw=randrw --rwmixread=50 --bs=4k --iodepth=64 --numjobs=2 --size=512MB --runtime=30 --gtod_reduce=1 --direct=1 --filename="/tmp/test.fio" --group_reporting --minimal
					cmd2 := exec.Command("timeout", "35", "sudo", "fio", "--name=rand_rw_"+BS, "--ioengine=libaio", "--rw=randrw", "--rwmixread=50", "--bs="+BS, "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=30", "--gtod_reduce=1", "--direct=1", "--filename=\""+buildPath+"/test.fio\"", "--group_reporting", "--minimal")
					output, err := cmd2.Output()
					if err == nil {
						tempText := string(output)
						tempList := strings.Split(tempText, "\n")
						for _, l := range tempList {
							if strings.Contains(l, "rand_rw_"+BS) {
								tpList := strings.Split(l, ";")
								// IOPS
								DISK_IOPS_R := tpList[8]
								DISK_IOPS_W := tpList[49]
								DISK_IOPS_R_INT, _ := strconv.Atoi(DISK_IOPS_R)
								DISK_IOPS_W_INT, _ := strconv.Atoi(DISK_IOPS_W)
								DISK_IOPS := DISK_IOPS_R_INT + DISK_IOPS_W_INT
								// Speed
								DISK_TEST_R := tpList[7]
								DISK_TEST_W := tpList[48]
								DISK_TEST_R_INT, _ := strconv.ParseFloat(DISK_TEST_R, 64)
								DISK_TEST_W_INT, _ := strconv.ParseFloat(DISK_TEST_W, 64)
								DISK_TEST := DISK_TEST_R_INT + DISK_TEST_W_INT
								// 拼接输出文本
								result += fmt.Sprintf("%-10s", buildPath) + "    "
								result += fmt.Sprintf("%-5s", BS) + "    "
								result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_R, "string")+"("+formatIOPS(DISK_IOPS_R, "string")+")") + "    "
								result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_W, "string")+"("+formatIOPS(DISK_IOPS_W, "string")+")") + "    "
								result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST, "float64")+"("+formatIOPS(DISK_IOPS, "int")+")") + "    "
								result += "\n"
							}
						}
					}
				}
			}
		}
	} else {
		tempText, err := buildFioFile(testPath, fioSize)
		defer os.Remove(testPath + "/test.fio")
		if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") {
			return tempText
		}
		// 测试
		blockSizes := []string{"4k", "64k", "512k", "1m"}
		for _, BS := range blockSizes {
			// timeout 35 fio --name=rand_rw_4k --ioengine=libaio --rw=randrw --rwmixread=50 --bs=4k --iodepth=64 --numjobs=2 --size=512MB --runtime=30 --gtod_reduce=1 --direct=1 --filename="/tmp/test.fio" --group_reporting --minimal
			cmd2 := exec.Command("timeout", "35", "sudo", "fio", "--name=rand_rw_"+BS, "--ioengine=libaio", "--rw=randrw", "--rwmixread=50", "--bs="+BS, "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=30", "--gtod_reduce=1", "--direct=1", "--filename=\""+testPath+"/test.fio\"", "--group_reporting", "--minimal")
			output, err := cmd2.Output()
			if err == nil {
				tempText := string(output)
				tempList := strings.Split(tempText, "\n")
				for _, l := range tempList {
					if strings.Contains(l, "rand_rw_"+BS) {
						tpList := strings.Split(l, ";")
						// IOPS
						DISK_IOPS_R := tpList[8]
						DISK_IOPS_W := tpList[49]
						DISK_IOPS_R_INT, _ := strconv.Atoi(DISK_IOPS_R)
						DISK_IOPS_W_INT, _ := strconv.Atoi(DISK_IOPS_W)
						DISK_IOPS := DISK_IOPS_R_INT + DISK_IOPS_W_INT
						// Speed
						DISK_TEST_R := tpList[7]
						DISK_TEST_W := tpList[48]
						DISK_TEST_R_INT, _ := strconv.ParseFloat(DISK_TEST_R, 64)
						DISK_TEST_W_INT, _ := strconv.ParseFloat(DISK_TEST_W, 64)
						DISK_TEST := DISK_TEST_R_INT + DISK_TEST_W_INT
						// 拼接输出文本
						result += fmt.Sprintf("%-10s", testPath) + "    "
						result += fmt.Sprintf("%-5s", BS) + "    "
						result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_R, "string")+"("+formatIOPS(DISK_IOPS_R, "string")+")") + "    "
						result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_W, "string")+"("+formatIOPS(DISK_IOPS_W, "string")+")") + "    "
						result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST, "float64")+"("+formatIOPS(DISK_IOPS, "int")+")") + "    "
						result += "\n"
					}
				}
			}
		}
	}
	return result
}
