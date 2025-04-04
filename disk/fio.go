package disk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	. "github.com/oneclickvirt/defaultset"
	"github.com/oneclickvirt/fio"
	"github.com/shirou/gopsutil/disk"
)

// FioTest 通过fio测试硬盘
func FioTest(language string, enableMultiCheck bool, testPath string) string {
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("开始FIO测试硬盘")
	}
	var (
		result, fioSize string
		devices         []string
		mountPoints     []string
	)
	parts, err := disk.Partitions(false)
	if EnableLoger {
		Logger.Info("识别到的磁盘分区:")
		for _, part := range parts {
			Logger.Info("路径: " + part.Mountpoint + ", 设备: " + part.Device)
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
		result += "Test Path     Block    Read(IOPS)              Write(IOPS)             Total(IOPS)\n"
	} else {
		result += "测试路径      块大小   读测试(IOPS)            写测试(IOPS)            总和(IOPS)\n"
	}
	// 生成测试文件
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		fioSize = "512M"
	} else {
		fioSize = "2G"
	}
	loggerInsert(Logger, "FIO测试文件大小: "+fioSize)
	if testPath == "" {
		if enableMultiCheck {
			loggerInsert(Logger, "开始多路径FIO测试")
			for index, path := range mountPoints {
				loggerInsert(Logger, "测试路径: "+path+", 设备: "+devices[index])
				buildOutput, err := buildFioFile(path, fioSize)
				defer os.Remove(path + "/test.fio")
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
			loggerInsert(Logger, "开始单路径FIO测试(/root或/tmp)")
			var buildPath string
			tempText, err := buildFioFile("/root", fioSize)
			defer os.Remove("/root/test.fio")
			if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") {
				if EnableLoger {
					Logger.Info("在/root路径生成FIO测试文件失败，尝试/tmp路径")
					if err != nil {
						Logger.Info("错误: " + err.Error())
					}
					if tempText != "" {
						Logger.Info("输出: " + tempText)
					}
				}
				buildOutput, err := buildFioFile("/tmp", fioSize)
				defer os.Remove("/tmp/test.fio")
				if err == nil {
					buildPath = "/tmp"
					if EnableLoger && buildOutput != "" {
						Logger.Info("在/tmp路径生成FIO测试文件输出: " + buildOutput)
					}
				} else if EnableLoger {
					Logger.Info("在/tmp路径生成FIO测试文件失败: " + err.Error())
				}
			} else {
				buildPath = "/root"
				if EnableLoger && tempText != "" {
					Logger.Info("在/root路径生成FIO测试文件输出: " + tempText)
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
		}
	} else {
		loggerInsert(Logger, "测试指定路径: "+testPath)
		tempText, err := buildFioFile(testPath, fioSize)
		defer os.Remove(testPath + "/test.fio")
		if err != nil || strings.Contains(tempText, "failed") || strings.Contains(tempText, "Permission denied") {
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
	args = append(args, "--name=setup", "--ioengine="+checkFioIOEngine(), "--rw=read", "--bs=64k", "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=1", "--gtod_reduce=1", "--filename="+path+"/test.fio", "--direct=1", "--minimal")
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
	// 测试
	blockSizes := []string{"4k", "64k", "512k", "1m"}
	for _, BS := range blockSizes {
		loggerInsert(Logger, "开始测试块大小: "+BS)
		// 构建命令参数
		var args []string
		if commandExists("timeout") {
			args = append(args, "35")
		}
		fioArgs := []string{"--name=rand_rw_" + BS, "--ioengine=" + ioEngine, "--rw=randrw", "--rwmixread=50", "--bs=" + BS, "--iodepth=64", "--numjobs=2", "--size=" + fioSize, "--runtime=30", "--gtod_reduce=1", "--direct=1", "--filename=" + path + "/test.fio", "--group_reporting", "--minimal"}
		if commandExists("timeout") {
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
			// 拼接输出文本
			result += fmt.Sprintf("%-10s", devicename) + "    "
			result += fmt.Sprintf("%-5s", BS) + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_R, "string")+"("+formatIOPS(DISK_IOPS_R, "string")+")") + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST_W, "string")+"("+formatIOPS(DISK_IOPS_W, "string")+")") + "    "
			result += fmt.Sprintf("%-20s", formatSpeed(DISK_TEST, "float64")+"("+formatIOPS(DISK_IOPS, "int")+")") + "    "
			result += "\n"
		}
	}
	return result
}
