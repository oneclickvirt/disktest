#!/bin/bash
#From https://github.com/oneclickvirt/disktest
#2024.08.05

rm -rf /usr/bin/disktest
rm -rf disktest
os=$(uname -s)
arch=$(uname -m)

check_cdn() {
  local o_url=$1
  for cdn_url in "${cdn_urls[@]}"; do
    if curl -sL -k "$cdn_url$o_url" --max-time 6 | grep -q "success" >/dev/null 2>&1; then
      export cdn_success_url="$cdn_url"
      return
    fi
    sleep 0.5
  done
  export cdn_success_url=""
}

check_cdn_file() {
  check_cdn "https://raw.githubusercontent.com/spiritLHLS/ecs/main/back/test"
  if [ -n "$cdn_success_url" ]; then
    echo "CDN available, using CDN"
  else
    echo "No CDN available, no use CDN"
  fi
}

cdn_urls=("https://cdn0.spiritlhl.top/" "http://cdn3.spiritlhl.net/" "http://cdn1.spiritlhl.net/" "http://cdn2.spiritlhl.net/")
check_cdn_file

download_file() {
    local url="$1"
    local output="$2"
    
    if ! wget -O "$output" "$url"; then
        echo "wget failed, trying curl..."
        if ! curl -L -o "$output" "$url"; then
            echo "Both wget and curl failed. Unable to download the file."
            return 1
        fi
    fi
    return 0
}

get_disktest_url() {
    local os="$1"
    local arch="$2"
    
    case $os in
        Linux|FreeBSD|OpenBSD)
            os_lower=$(echo $os | tr '[:upper:]' '[:lower:]')
            case $arch in
                "x86_64"|"x86"|"amd64"|"x64") arch_name="amd64" ;;
                "i386"|"i686") arch_name="386" ;;
                "armv7l"|"armv8"|"armv8l"|"aarch64"|"arm64") arch_name="arm64" ;;
                *) echo "Unsupported architecture: $arch" && return 1 ;;
            esac
            echo "${cdn_success_url}https://github.com/oneclickvirt/disktest/releases/download/output/disktest-${os_lower}-${arch_name}"
            ;;
        Darwin)
            case $arch in
                "x86_64"|"x86"|"amd64"|"x64") arch_name="amd64" ;;
                "i386"|"i686") arch_name="386" ;;
                "armv7l"|"armv8"|"armv8l"|"aarch64"|"arm64") arch_name="arm64" ;;
                *) echo "Unsupported architecture: $arch" && return 1 ;;
            esac
            echo "https://github.com/oneclickvirt/disktest/releases/download/output/disktest-darwin-${arch_name}"
            ;;
        *)
            echo "Unsupported operating system: $os"
            return 1
            ;;
    esac
}

url=$(get_disktest_url "$os" "$arch")
if [ $? -eq 0 ]; then
    if download_file "$url" "disktest"; then
        echo "Successfully downloaded disktest"
    else
        echo "Failed to download disktest"
        exit 1
    fi
else
    exit 1
fi

chmod 777 disktest
cp disktest /usr/bin/disktest
