#!/bin/bash
#From https://github.com/oneclickvirt/diskTest

rm -rf diskTest
os=$(uname -s)
arch=$(uname -m)

case $os in
  Linux)
    case $arch in
      "x86_64" | "x86" | "amd64" | "x64")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-linux-amd64
        ;;
      "i386" | "i686")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-linux-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-linux-arm64
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
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-darwin-amd64
        ;;
      "i386" | "i686")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-darwin-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-darwin-arm64
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
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-freebsd-amd64
        ;;
      "i386" | "i686")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-freebsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-freebsd-arm64
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
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-openbsd-amd64
        ;;
      "i386" | "i686")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-openbsd-386
        ;;
      "armv7l" | "armv8" | "armv8l" | "aarch64")
        wget -O diskTest https://github.com/oneclickvirt/diskTest/releases/download/output/diskTest-openbsd-arm64
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

chmod 777 diskTest
./diskTest