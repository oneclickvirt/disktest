# diskTest

[![Hits](https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2FdiskTest&count_bg=%2379C83D&title_bg=%23555555&icon=sonarcloud.svg&icon_color=%2345FFC2&title=hits&edge_flat=false)](https://hits.seeyoufarm.com) [![Build and Release](https://github.com/oneclickvirt/diskTest/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/diskTest/actions/workflows/main.yaml)

硬盘IO测试 (Disk IO Test)

开发中，不要使用

## 功能

- [x] winstat测试
- [x] dd测试
- [x] fio测试
- [x] 支持双语输出，以```-l```指定```zh```或```en```可指定输出的语言，未指定时默认使用中文输出
- [x] 支持指定测试方式，以```-m```指定```dd```或```fio```指定测试方式，未指定时默认使用```dd```进行测试
- [x] 支持单/多盘IO测试，以```-d```指定```single```或```multi```可指定是否测试多盘，未指定时默认仅测试单盘```/root```或```C:```路径
- [x] 支持指定路径IO测试，以```-p```指定路径
- [ ] 测试前需检测剩余硬盘大小是否支持进行测试
- [x] 全平台编译支持

## 使用

更新时间：2024.05.12

```shell
curl https://raw.githubusercontent.com/oneclickvirt/diskTest/main/diskTest_install.sh -sSf | sh
```

更多架构请查看 https://github.com/oneclickvirt/diskTest/releases/tag/output
