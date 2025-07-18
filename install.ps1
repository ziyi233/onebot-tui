# install.ps1
# This script downloads and installs the latest release of onebot-tui for Windows.

$ErrorActionPreference = 'Stop'

$REPO = "ziyi233/onebot-tui"

function Get-LatestRelease {
    $url = "https://api.github.com/repos/$REPO/releases/latest"
    try {
        $response = Invoke-RestMethod -Uri $url
        return $response.tag_name.TrimStart('v')
    } catch {
        Write-Error "Failed to get latest release from GitHub: $_"
        exit 1
    }
}

function Get-Arch {
    $arch = $env:PROCESSOR_ARCHITECTURE
    if ($arch -eq "AMD64") {
        return "amd64"
    } elseif ($arch -eq "ARM64") {
        return "arm64"
    }
    Write-Error "Unsupported architecture: $arch"
    exit 1
}

function Main {
    $version = Get-LatestRelease
    $os = "windows"
    $arch = Get-Arch

    $fileName = "onebot-tui_${version}_${os}_${arch}.zip"
    $downloadUrl = "https://github.com/$REPO/releases/download/v${version}/${fileName}"

    Write-Host "Downloading onebot-tui v$version for $os/$arch..."
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $fileName
    } catch {
        Write-Error "Failed to download release asset: $_"
        exit 1
    }

    Write-Host "Unzipping..."
    Expand-Archive -Path $fileName -DestinationPath . -Force
    Remove-Item $fileName

    Write-Host ""
    Write-Host "onebot-tui has been installed successfully!"
    Write-Host "You can now run the daemon with .\onebot-tui-daemon.exe"
}

Main
