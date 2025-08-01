package disk

import (
	"strings"
	
	"github.com/shirou/gopsutil/disk"
	. "github.com/oneclickvirt/defaultset"
)

// TestPathInfo 测试路径信息结构体
type TestPathInfo struct {
	Devices     []string // 设备列表
	MountPoints []string // 挂载点列表
}

// getTestPaths 获取可用的测试路径，返回设备和挂载点列表
func getTestPaths() (TestPathInfo, error) {
	var pathInfo TestPathInfo
	parts, err := disk.Partitions(false)
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("识别到的磁盘分区:")
		for _, part := range parts {
			Logger.Info("路径: " + part.Mountpoint + ", 设备: " + part.Device)
		}
	}
	if err == nil {
		for _, f := range parts {
			// 过滤掉不需要的设备
			if !strings.Contains(f.Device, "vda") && 
			   !strings.Contains(f.Device, "snap") && 
			   !strings.Contains(f.Device, "loop") {
				if isWritableMountpoint(f.Mountpoint) {
					pathInfo.Devices = append(pathInfo.Devices, f.Device)
					pathInfo.MountPoints = append(pathInfo.MountPoints, f.Mountpoint)
					loggerInsert(Logger, "添加可写分区: "+f.Mountpoint+", 设备: "+f.Device)
				}
			}
		}
	}
	return pathInfo, err
}