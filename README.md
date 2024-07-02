# disktest

[![Hits](https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fdisktest&count_bg=%2379C83D&title_bg=%23555555&icon=sonarcloud.svg&icon_color=%2345FFC2&title=hits&edge_flat=false)](https://hits.seeyoufarm.com) [![Build and Release](https://github.com/oneclickvirt/disktest/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/disktest/actions/workflows/main.yaml)

硬盘IO测试 (Disk IO Test)

## 功能

- [x] 使用```winsat```测试
- [x] 使用```dd```测试
- [x] 使用```fio```测试
- [x] 支持双语输出，以```-l```指定```zh```或```en```可指定输出的语言，未指定时默认使用中文输出
- [x] 支持指定测试方式，以```-m```指定```dd```或```fio```指定测试方式，未指定时默认使用```dd```进行测试
- [x] 支持单/多盘IO测试，以```-d```指定```single```或```multi```可指定是否测试多盘，未指定时默认仅测试单盘```/root```或```C:```路径
- [x] 支持指定路径IO测试，以```-p```指定路径
- [x] 全平台编译支持

注意：默认不自动安装```fio```组件，如需使用请自行安装后再使用本项目，如```apt update && apt install fio -y```

不使用```sysbench```进行硬盘IO测试，因为默认设置下```fio```测试效果比```sysbench```测试更优，且```fio```的维护者比```sysbench```的维护者更活跃。

## TODO

- [ ] 正式测试前检测当前路径挂载盘剩余空间是否足够生成测试文件
- [ ] fio测试在WIN系统中匹配自动下载exe文件调用
- [ ] 优化测试失败时的报错和输出

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
  -m string
        Specific Test Method (dd or fio)
  -p string
        Specific Test Disk Path (default is /root or C:)
  -v    Show version
```

有环境依赖，Linux/Unix相关系统请确保本地至少安装有```dd```或```fio```工具其中之一，更多架构请查看 https://github.com/oneclickvirt/disktest/releases/tag/output

dd测试：

![图片](https://github.com/oneclickvirt/disktest/assets/103393591/163b1150-dc45-4d53-abbf-c6e1acca4e19)

fio测试：

![图片](https://github.com/oneclickvirt/disktest/assets/103393591/3052b430-2d93-4a07-9e12-0a911ffb36c3)

winsat测试：

![1716466264315](https://github.com/oneclickvirt/disktest/assets/103393591/505b9525-216c-4e9a-b602-65382177d414)

## 卸载

```
rm -rf /root/disktest
rm -rf /usr/bin/disktest
```

## 在Golang中使用

```
go get github.com/oneclickvirt/disktest@latest
```
