# disktest

[![Hits](https://hits.spiritlhl.net/disktest.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false)](https://hits.spiritlhl.net)

[![Build and Release](https://github.com/oneclickvirt/disktest/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/disktest/actions/workflows/main.yaml)

硬盘IO测试 (Disk IO Test)

## 功能

- [x] 支持使用```winsat```测试
- [x] 支持使用```dd```测试
- [x] 支持使用```fio```测试，支持自动选择IO引擎测试，测试优先级为```libaio[仅linux] > posixaio > psync```
- [x] 支持Go自身静态依赖注入[fio](https://github.com/oneclickvirt/fio)和[dd](https://github.com/oneclickvirt/dd)，使用时无额外环境依赖需求
- [x] 支持双语输出，以```-l```指定```zh```或```en```可指定输出的语言，未指定时默认使用中文输出
- [x] 支持指定测试方式，以```-m```指定```dd```或```fio```指定测试方式，未指定时默认使用```fio```进行测试
- [x] 支持单/多盘IO测试，以```-d```指定```single```或```multi```可指定是否测试多盘，未指定时默认仅测试单盘```/root```或```C:```路径
- [x] 支持指定路径IO测试，以```-p```指定路径
- [x] 正式测试前检测当前路径挂载盘剩余空间是否足够生成测试文件
- [x] 全平台编译支持，适配MACOS系统等无root权限等环境进行测试

PS: 不使用```sysbench```进行硬盘IO测试，因为默认设置下```fio```测试效果比```sysbench```测试更贴近机器本身的性能，且```fio```的维护者比```sysbench```的维护者更活跃。

## TODO

- [ ] 修复WIN系统的虚拟下的disk测试无法使用winsat的问题

## 使用

下载、安装、更新

```shell
curl https://raw.githubusercontent.com/oneclickvirt/disktest/main/disktest_install.sh -sSf | bash
```

或

```shell
curl https://cdn.spiritlhl.net/https://raw.githubusercontent.com/oneclickvirt/disktest/main/disktest_install.sh -sSf | bash
```

使用

```
disktest
```

或

```
./disktest
```

进行测试

```
Usage: disktest [options]
  -d string
        Enable multi disk check parameter (single or multi, default is single)
  -h    Show help information
  -l string
        Language parameter (en or zh)
  -log
        Enable logging
  -m string
        Specific Test Method (dd or fio)
  -p string
        Specific Test Disk Path (default is /root or C:)
  -v    Show version
```

更多架构请查看 https://github.com/oneclickvirt/disktest/releases/tag/output

## 卸载

```
rm -rf /root/disktest
rm -rf /usr/bin/disktest
```

## 在Golang中使用

```
go get github.com/oneclickvirt/disktest@v0.0.8-20250701082736
```

## 测试图

dd测试：

![图片](https://github.com/oneclickvirt/disktest/assets/103393591/163b1150-dc45-4d53-abbf-c6e1acca4e19)

fio测试：

![图片](https://github.com/oneclickvirt/disktest/assets/103393591/3052b430-2d93-4a07-9e12-0a911ffb36c3)

winsat测试：

![1716466264315](https://github.com/oneclickvirt/disktest/assets/103393591/505b9525-216c-4e9a-b602-65382177d414)
