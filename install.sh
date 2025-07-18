#!/bin/sh
set -e

# This script downloads and installs the latest release of onebot-tui.

REPO="ziyi233/onebot-tui"

get_latest_release() {
  curl --silent "https://api.github.com/repos/$REPO/releases/latest" | # Get latest release from GitHub api
    grep '"tag_name":' |                                            # Get tag line
    sed -E 's/.*"v([^"]+)".*/\1/'                               # Pluck version number
}

get_arch() {
  ARCH=$(uname -m)
  case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64) ARCH="arm64" ;;
  esac
  echo $ARCH
}

get_os() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  echo $OS
}

main() {
  VERSION=$(get_latest_release)
  OS=$(get_os)
  ARCH=$(get_arch)

  FILENAME="onebot-tui_${VERSION}_${OS}_${ARCH}.zip"
  DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/${FILENAME}"

  echo "Downloading onebot-tui v${VERSION} for ${OS}/${ARCH}..."
  curl -L -o "${FILENAME}" "${DOWNLOAD_URL}"

  echo "Unzipping..."
  unzip -o "${FILENAME}"
  rm "${FILENAME}"

  echo "Making binaries executable..."
  chmod +x onebot-tui-daemon
  chmod +x onebot-tui-controller

  echo ""
  echo "onebot-tui has been installed successfully!"
  echo "You can now run the daemon with ./onebot-tui-daemon"
  echo ""
  echo "For system-wide access, move the binaries to your PATH, for example:"
  echo "  sudo mv onebot-tui-daemon onebot-tui-controller /usr/local/bin/"
}

main
