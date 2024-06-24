#!/bin/bash
#From https://github.com/oneclickvirt/disktest
#2024.06.24

rm -rf /usr/bin/disktest
os=$(uname -s)
arch=$(uname -m)

case $os in
  Linux)
    case $arch in
      "x86_64" | "x86" | "amd64" | "x64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-linux-amd64
        ;;
      "i386" | "i686")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-linux-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-linux-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  Darwin)
    case $arch in
      "x86_64" | "x86" | "amd64" | "x64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-darwin-amd64
        ;;
      "i386" | "i686")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-darwin-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-darwin-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  FreeBSD)
    case $arch in
      amd64)
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-freebsd-amd64
        ;;
      "i386" | "i686")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-freebsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-freebsd-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  OpenBSD)
    case $arch in
      amd64)
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-openbsd-amd64
        ;;
      "i386" | "i686")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-openbsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64" | "arm64")
        wget -O disktest https://github.com/oneclickvirt/disktest/releases/download/output/disktest-openbsd-arm64
        ;;
      *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
    esac
    ;;
  *)
    echo "Unsupported operating system: $os"
    exit 1
    ;;
esac

chmod 777 disktest
cp disktest /usr/bin/disktest
