package disktest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// WinsatTest 通过windows自带系统工具测试IO
func WinsatTest(language string, enableMultiCheck bool) string {
	var result string
	parts, err := disk.Partitions(true)
	if err == nil {
		if language == "zh" {
			result += "测试的硬盘                随机写入                  顺序读取                 顺序写入\n"
		} else {
			result += "Test Disk               Random Read             Sequential Read         Sequential Write\n"
		}
		if enableMultiCheck {
			for _, f := range parts {
				// winsat disk -drive 硬盘名字
				cmd := exec.Command("winsat", "disk", "-drive", f.Device)
				output, err := cmd.Output()
				if err == nil {
					result += fmt.Sprintf("%-20s", f.Device) + "    "
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
				}
			}
		} else {
			cmd := exec.Command("winsat", "disk", "-drive", "C:")
			output, err := cmd.Output()
			if err == nil {
				result += fmt.Sprintf("%-20s", "C:") + "    "
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
	if enableMultiCheck {
		if language == "zh" {
			result += "测试盘        块大小             直接写入                           直接读取\n"
		} else {
			result += "Test Disk    Block Size         Direct Write                      Direct Read\n"
		}
		for index, path := range mountPoints {
			// 写入测试
			// dd if=/dev/zero of=/tmp/100MB.test bs=4k count=25600 oflag=direct
			cmd1 := exec.Command("dd", "if=/dev/zero", "of="+path+"/100MB.test", "bs=4k", "count=25600", "oflag=direct")
			stderr1, err := cmd1.StderrPipe()
			if err == nil {
				result += fmt.Sprintf("%-10s", strings.TrimSpace(devices[index])) + "    " + fmt.Sprintf("%-15s", "100MB-4K Block") + "    "
				if err := cmd1.Start(); err == nil {
					outputBytes, err := io.ReadAll(stderr1)
					if err == nil {
						tempText := string(outputBytes)
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
					}
				}
			}
			// 读取测试
			// dd if=/tmp/100MB.test of=/dev/null bs=4k count=25600 oflag=direct
			nullPath := "/dev/null"
			_, err = os.Stat(nullPath)
			var cmd2 *exec.Cmd
			if os.IsNotExist(err) {
				cmd2 = exec.Command("dd", "if="+path+"/100MB.test", "of="+path+"/100MB_read.test", "bs=4k", "count=25600", "oflag=direct")
			} else {
				cmd2 = exec.Command("dd", "if="+path+"/100MB.test", "of=/dev/null", "bs=4k", "count=25600", "oflag=direct")
			}
			stderr2, err := cmd2.StderrPipe()
			if err == nil {
				if err := cmd2.Start(); err == nil {
					outputBytes, err := io.ReadAll(stderr2)
					if err == nil {
						tempText := string(outputBytes)
						fmt.Println("1", tempText)
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
					}
				}
			}
			result += "\n"
		}
	} else {
		cmd := exec.Command("dd", "if=/dev/zero", "of=/root/100MB.test", "bs=4k", "count=25600", "oflag=direct")
		stderr, err := cmd.StderrPipe()
		if err == nil {
			if err := cmd.Start(); err == nil {
				outputBytes, err := io.ReadAll(stderr)
				if err == nil {
					tempText := string(outputBytes)
					fmt.Println(tempText)
				}
			}
		}
	}
	//25600+0 records in
	//25600+0 records out
	//104857600 bytes (105 MB, 100 MiB) copied, 15.034 s, 7.0 MB/s
	// dd if=/dev/zero of=/root/1GB.test bs=1M count=1000 oflag=direct

	//1000+0 records in
	//1000+0 records out
	//1048576000 bytes (1.0 GB, 1000 MiB) copied, 2.7358 s, 383 MB/s
	// rm -rf 1GB.test 100MB.test
	return result
}
