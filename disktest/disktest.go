package disktest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// WinsatTest 通过windows自带系统工具测试IO
func WinsatTest(language string, enableMultiCheck bool) string {
	var result string
	parts, err := disk.Partitions(true)
	if err == nil {
		if language == "en" {
			result += "Test Disk               Random Read             Sequential Read         Sequential Write\n"
		} else {
			result += "测试的硬盘                随机写入                  顺序读取                 顺序写入\n"
		}
		if enableMultiCheck {
			for _, f := range parts {
				result += fmt.Sprintf("%-20s", f.Device) + "    "
				result += getDiskPerformance(f.Device)
			}
		} else {
			result += fmt.Sprintf("%-20s", "C:") + "    "
			result += getDiskPerformance("C:")
		}
	}
	return result
}

// DDTest 通过 dd 命令测试硬盘IO
func DDTest(language string, enableMultiCheck bool) string {
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
	if enableMultiCheck {
		for index, path := range mountPoints {
			// 写入测试
			// dd if=/dev/zero of=/tmp/100MB.test bs=4k count=25600 oflag=direct
			cmd1 := exec.Command("dd", "if=/dev/zero", "of="+path+"/100MB.test", "bs=4k", "count=25600", "oflag=direct")
			defer os.Remove(path + "/100MB.test")
			stderr1, err := cmd1.StderrPipe()
			if err == nil {
				result += fmt.Sprintf("%-10s", strings.TrimSpace(devices[index])) + "    " + fmt.Sprintf("%-15s", "100MB-4K Block") + "    "
				if err := cmd1.Start(); err == nil {
					outputBytes, err := io.ReadAll(stderr1)
					if err == nil {
						tempText := string(outputBytes)
						result += parseResultDD(tempText)
					}
				}
			}
			os.Remove(path + "/100MB.test")
			// 读取测试
			// dd if=/tmp/100MB.test of=/dev/null bs=4k count=25600 oflag=direct
			cmd2 := exec.Command("dd", "if="+path+"/100MB.test", "of=/dev/null", "bs=4k", "count=25600", "oflag=direct")
			defer os.Remove(path + "/100MB.test")
			stderr2, err := cmd2.StderrPipe()
			if err == nil {
				if err := cmd2.Start(); err == nil {
					outputBytes, err := io.ReadAll(stderr2)
					if err == nil {
						tempText := string(outputBytes)
						if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
							cmd2 = exec.Command("dd", "if="+path+"/100MB.test", "of="+path+"/100MB_read.test", "bs=4k", "count=25600", "oflag=direct")
							defer os.Remove(path + "/100MB.test")
							defer os.Remove(path + "/100MB_read.test")
							stderr2, err = cmd2.StderrPipe()
							if err == nil {
								if err := cmd2.Start(); err == nil {
									outputBytes, err := io.ReadAll(stderr2)
									if err == nil {
										tempText = string(outputBytes)
									}
								}
							}
						}
						result += parseResultDD(tempText)
					}
				}
			}
			os.Remove(path + "/100MB.test")
			os.Remove(path + "/100MB_read.test")
			result += "\n"
		}
	} else {
		// 写入测试
		cmd1 := exec.Command("dd", "if=/dev/zero", "of=/root/100MB.test", "bs=4k", "count=25600", "oflag=direct")
		defer os.Remove("/root/100MB.test")
		stderr, err := cmd1.StderrPipe()
		if err == nil {
			if err := cmd1.Start(); err == nil {
				outputBytes, err := io.ReadAll(stderr)
				if err == nil {
					tempText := string(outputBytes)
					if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
						if err == nil {
							cmd1 = exec.Command("dd", "if=/tmp/100MB.test", "of=/tmp/100MB_read.test", "bs=4k", "count=25600", "oflag=direct")
							defer os.Remove("/tmp/100MB.test")
							defer os.Remove("/tmp/100MB_read.test")
							stderr, err = cmd1.StderrPipe()
							if err == nil {
								if err := cmd1.Start(); err == nil {
									outputBytes, err := io.ReadAll(stderr)
									if err == nil {
										tempText = string(outputBytes)
										result += fmt.Sprintf("%-10s", "/tmp") + "    " + fmt.Sprintf("%-15s", "100MB-4K Block") + "    "
									}
								}
							}
						}
					} else {
						result += fmt.Sprintf("%-10s", "/root") + "    " + fmt.Sprintf("%-15s", "100MB-4K Block") + "    "
					}
					result += parseResultDD(tempText)
				}
			}
		}
		// 读取测试
		cmd2 := exec.Command("dd", "if=/root/100MB.test", "of=/dev/null", "bs=4k", "count=25600", "oflag=direct")
		defer os.Remove("/root/100MB.test")
		stderr2, err := cmd2.StderrPipe()
		if err == nil {
			if err := cmd2.Start(); err == nil {
				outputBytes, err := io.ReadAll(stderr2)
				if err == nil {
					tempText := string(outputBytes)
					if strings.Contains(tempText, "Invalid argument") || strings.Contains(tempText, "Permission denied") {
						cmd2 = exec.Command("dd", "if=/tmp/100MB.test", "of=/tmp/100MB_read.test", "bs=4k", "count=25600", "oflag=direct")
						defer os.Remove("/tmp/100MB.test")
						defer os.Remove("/tmp/100MB_read.test")
						stderr2, err = cmd2.StderrPipe()
						if err == nil {
							if err := cmd2.Start(); err == nil {
								outputBytes, err := io.ReadAll(stderr2)
								if err == nil {
									tempText = string(outputBytes)
								}
							}
						}
					}
					result += parseResultDD(tempText)
				}
			}
		}
		result += "\n"
	}
	return result
}

//25600+0 records in
//25600+0 records out
//104857600 bytes (105 MB, 100 MiB) copied, 15.034 s, 7.0 MB/s

//1000+0 records in
//1000+0 records out
//1048576000 bytes (1.0 GB, 1000 MiB) copied, 2.7358 s, 383 MB/s

// FioTest 通过fio测试硬盘
func FioTest(language string, enableMultiCheck bool) string {
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
	if language == "en" {
		result += "Test Path    Block Size         4k (IOPS)                        64k(IOPS)\n"
	} else {
		result += "测试路径      块大小             4k (IOPS)                        64k(IOPS)\n"
	}
	// 生成测试文件
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "x86" || runtime.GOARCH == "x86_64" {
		fioSize = "2G"
	} else {
		fioSize = "512MB"
	}
	if enableMultiCheck {
		for index, path := range mountPoints {
			// 生成对应文件
			// fio --name=setup --ioengine=libaio --rw=read --bs=64k --iodepth=64 --numjobs=2 --size=512MB --runtime=1 --gtod_reduce=1 --filename="/tmp/test.fio" --direct=1 --minimal
			cmd1 := exec.Command("fio", "--name=setup", "--ioengine=libaio", "--rw=read", "--bs=64k", "--iodepth=64", "--numjobs=2", "--size="+fioSize, "--runtime=1", "--gtod_reduce=1",
				"--filename=\""+path+"/test.fio\"", "--direct=1", "--minimal")
			defer os.Remove(path + "/test.fio")
			stderr1, err := cmd1.StderrPipe()
			if err == nil {
				// result += fmt.Sprintf("%-10s", strings.TrimSpace(devices[index])) + "    " + fmt.Sprintf("%-15s", "100MB-4K Block") + "    "
				// if err := cmd1.Start(); err == nil {
				// 	outputBytes, err := io.ReadAll(stderr1)
				// 	if err == nil {
				// 		tempText := string(outputBytes)
				// 		result += parseResultDD(tempText)
				// 	}
				// }
			}
		}
	}
	return result
}
