<#
Haypile installer for Windows: detects your architecture, downloads the
latest release binary from GitHub, and installs it as `hay.exe`.

  irm https://haypile.sh/install.ps1 | iex

Nothing here phones home. The only network request is the download from
GitHub Releases, and you are reading the script that makes it.
#>
$ErrorActionPreference = 'Stop'

# PowerShell 5.1 defaults to TLS 1.0; GitHub needs 1.2+.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'BenyD/haypile'
$InstallDir = if ($env:HAY_INSTALL_DIR) { $env:HAY_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\hay' }

function Fail($msg) {
    Write-Host "install.ps1: $msg" -ForegroundColor Red
    exit 1
}

# Only windows/amd64 is released; on arm64 Windows the amd64 build runs
# under emulation, so the amd64 archive is always the right download.
$arch = 'amd64'
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
    Write-Host 'Note: no native arm64 build yet; installing the amd64 binary (runs under emulation).'
}

$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ 'User-Agent' = 'haypile-installer' }
$version = $release.tag_name
if (-not $version) { Fail 'could not determine the latest release' }
$plain = $version.TrimStart('v')
$url = "https://github.com/$Repo/releases/download/$version/haypile_${plain}_windows_${arch}.zip"

Write-Host "Installing hay $version for windows/$arch..."

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("hay-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
    $zip = Join-Path $tmp 'hay.zip'
    Invoke-WebRequest -Uri $url -OutFile $zip -Headers @{ 'User-Agent' = 'haypile-installer' }
    Expand-Archive -Path $zip -DestinationPath $tmp -Force

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path (Join-Path $tmp 'hay.exe') -Destination (Join-Path $InstallDir 'hay.exe') -Force
}
finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# Put hay on the user PATH (no admin needed) for future terminals, and this
# session, if it is not already there.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $InstallDir) {
    $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "Added $InstallDir to your user PATH."
}
if (($env:Path -split ';') -notcontains $InstallDir) {
    $env:Path = "$env:Path;$InstallDir"
}

Write-Host "Installed: $(Join-Path $InstallDir 'hay.exe')"
Write-Host ''
Write-Host 'Get started (open a new terminal if hay is not found):'
Write-Host '  hay add $env:USERPROFILE\Documents'
Write-Host '  hay search "something you remember"'
