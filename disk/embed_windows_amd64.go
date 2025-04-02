//go:build windows && amd64
// +build windows,amd64

package disk

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/oneclickvirt/defaultset"
)

//go:embed bin/fio-windows-amd64.exe
var binFiles embed.FS

// getFioBinary 获取与当前系统匹配的 fio 二进制文件
func getFioBinary() (string, error) {
	binaryName := "fio-windows-amd64.exe"
	// 创建临时目录存放二进制文件
	tempDir, err := os.MkdirTemp("", "disktest")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}
	// 读取嵌入的二进制文件
	binPath := filepath.Join("bin", binaryName)
	fileContent, err := binFiles.ReadFile(binPath)
	if err != nil {
		return "", fmt.Errorf("读取嵌入的 fio 二进制文件失败: %v", err)
	}
	// 写入临时文件
	tempFile := filepath.Join(tempDir, binaryName)
	if err := os.WriteFile(tempFile, fileContent, 0755); err != nil {
		return "", fmt.Errorf("写入临时 fio 文件失败: %v", err)
	}
	if EnableLoger {
		Logger.Info("使用嵌入的 fio 二进制文件: " + tempFile)
	}
	return tempFile, nil
}
