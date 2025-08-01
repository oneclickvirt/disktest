package disk

import (
	"strings"

	. "github.com/oneclickvirt/defaultset"
	"github.com/shirou/gopsutil/disk"
)

// 同步 https://github.com/oneclickvirt/basics/blob/main/system/disk.go 的筛选和添加逻辑
var (
	expectDiskFsTypes = []string{
		"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "bcachefs", "btrfs",
		"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
	}
	excludeFsTypes = []string{
		"tmpfs", "devtmpfs", "sysfs", "proc", "devpts", "cgroup", "cgroup2",
		"pstore", "bpf", "tracefs", "debugfs", "mqueue", "hugetlbfs",
		"securityfs", "swap", "squashfs", "overlay", "aufs",
	}
	excludeMountPoints = []string{
		"/dev/shm", "/run", "/sys", "/proc", "/tmp", "/var/tmp",
		"/boot/efi", "/snap", "/var/lib/kubelet", "/var/lib/docker",
		"/var/lib/lxd", "/var/lib/incus", "/snap", "/vz/root",
	}
)

// TestPathInfo 测试路径信息结构体
type TestPathInfo struct {
	Devices     []string // 设备列表
	MountPoints []string // 挂载点列表
}

// shouldExcludeFsType 检查是否应该排除文件系统类型
func shouldExcludeFsType(fsType string) bool {
	fsType = strings.ToLower(fsType)
	for _, excludeType := range excludeFsTypes {
		if strings.Contains(fsType, excludeType) {
			return true
		}
	}
	return false
}

// shouldExcludeMountPoint 检查是否应该排除挂载点
func shouldExcludeMountPoint(mountPoint string) bool {
	for _, excludePoint := range excludeMountPoints {
		if strings.Contains(mountPoint, excludePoint) {
			// 特殊处理 /run 目录，如果空间足够大则不排除
			if strings.Contains(mountPoint, "/run") {
				if usage, err := disk.Usage(mountPoint); err == nil {
					fiftyGB := uint64(50 * 1024 * 1024 * 1024)
					if usage.Total > fiftyGB {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

// isExpectedFsType 检查是否是期望的文件系统类型
func isExpectedFsType(fsType string) bool {
	fsType = strings.ToLower(fsType)
	for _, expectedType := range expectDiskFsTypes {
		if strings.Contains(fsType, expectedType) {
			return true
		}
	}
	return false
}

// getTestPaths 获取可用的测试路径,返回设备和挂载点列表
func getTestPaths() (TestPathInfo, error) {
	var pathInfo TestPathInfo
	parts, err := disk.Partitions(false)
	if EnableLoger {
		InitLogger()
		defer Logger.Sync()
		Logger.Info("识别到的磁盘分区:")
		for _, part := range parts {
			Logger.Info("路径: " + part.Mountpoint + ", 设备: " + part.Device + ", 文件系统: " + part.Fstype)
		}
	}
	if err == nil {
		for _, f := range parts {
			fsType := strings.ToLower(f.Fstype)
			// 检查文件系统类型是否应该被排除
			if shouldExcludeFsType(fsType) {
				loggerInsert(Logger, "排除文件系统类型: "+f.Fstype+", 设备: "+f.Device+", 挂载点: "+f.Mountpoint)
				continue
			}
			// 检查挂载点是否应该被排除
			if shouldExcludeMountPoint(f.Mountpoint) {
				loggerInsert(Logger, "排除挂载点: "+f.Mountpoint+", 设备: "+f.Device)
				continue
			}
			// 设备过滤逻辑
			if strings.Contains(f.Device, "vda") ||
				strings.Contains(f.Device, "snap") ||
				strings.Contains(f.Device, "loop") {
				loggerInsert(Logger, "排除设备类型: "+f.Device+", 挂载点: "+f.Mountpoint)
				continue
			}
			// 优先选择期望的文件系统类型
			if isExpectedFsType(fsType) {
				if isWritableMountpoint(f.Mountpoint) {
					pathInfo.Devices = append(pathInfo.Devices, f.Device)
					pathInfo.MountPoints = append(pathInfo.MountPoints, f.Mountpoint)
					loggerInsert(Logger, "添加期望文件系统可写分区: "+f.Mountpoint+", 设备: "+f.Device+", 文件系统: "+f.Fstype)
				} else {
					loggerInsert(Logger, "期望文件系统但不可写: "+f.Mountpoint+", 设备: "+f.Device+", 文件系统: "+f.Fstype)
				}
			} else {
				// 其他文件系统类型，如果可写也加入
				if isWritableMountpoint(f.Mountpoint) {
					pathInfo.Devices = append(pathInfo.Devices, f.Device)
					pathInfo.MountPoints = append(pathInfo.MountPoints, f.Mountpoint)
					loggerInsert(Logger, "添加其他可写分区: "+f.Mountpoint+", 设备: "+f.Device+", 文件系统: "+f.Fstype)
				} else {
					loggerInsert(Logger, "其他文件系统但不可写: "+f.Mountpoint+", 设备: "+f.Device+", 文件系统: "+f.Fstype)
				}
			}
		}
	}
	return pathInfo, err
}
