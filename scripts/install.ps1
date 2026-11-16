# Installs the tollbit CLI from tollbit.com, or from GitHub Releases when
# tollbit.com can't be reached. -Repo set to another repository uses only
# GitHub. A checksum mismatch fails the install and never falls back.
[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$HOME\AppData\Local\Programs\tollbit\bin",
    [switch]$Force,
    [switch]$NoModifyPath,
    [switch]$PrintPathInstructions,
    [string]$Repo = "tollbit/cli",
    [string]$BaseUrl = "https://tollbit.com/.agents/cli"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$DefaultRepo = "tollbit/cli"

# A blocked or dropped connection to tollbit.com should fall back to GitHub
# quickly, not hang.
$MirrorTimeoutSec = 30

function Get-LatestTag {
    param([string]$Repository)
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest"
    return $release.tag_name
}

function Invoke-MirrorDownload {
    param([string]$Uri, [string]$OutFile)
    Invoke-WebRequest -Uri $Uri -OutFile $OutFile -TimeoutSec $MirrorTimeoutSec -UseBasicParsing
}

# Returns the version $BaseUrl/latest names (its version=vX.Y.Z line). Other
# lines are ignored.
function Get-MirrorLatestTag {
    param([string]$BaseUrl, [string]$TmpDir)
    $path = Join-Path $TmpDir "latest"
    Invoke-MirrorDownload -Uri "$BaseUrl/latest" -OutFile $path
    $line = Get-Content -Path $path | Where-Object { $_ -match '^version=' } | Select-Object -First 1
    $tag = if ($line) { $line.Substring("version=".Length).Trim() } else { "" }
    if (-not $tag.StartsWith("v")) {
        throw "$BaseUrl/latest has no version=v... line"
    }
    return $tag
}

function Get-ArchiveName {
    param([string]$Tag, [string]$Arch)
    return "tollbit_$($Tag.Substring(1))_windows_$Arch.zip"
}

function Get-ChecksumsName {
    param([string]$Tag)
    return "tollbit_$($Tag.Substring(1))_checksums.txt"
}

# Downloads the archive and checksums from $BaseUrl into $TmpDir and returns
# the version. Throws if anything can't be downloaded.
function Save-MirrorRelease {
    param([string]$Tag, [string]$Arch, [string]$BaseUrl, [string]$TmpDir)
    if ($Tag -eq "latest") {
        $Tag = Get-MirrorLatestTag -BaseUrl $BaseUrl -TmpDir $TmpDir
    }
    $archiveName = Get-ArchiveName -Tag $Tag -Arch $Arch
    $checksumsName = Get-ChecksumsName -Tag $Tag
    Write-Host "Downloading $archiveName from $BaseUrl..."
    Invoke-MirrorDownload -Uri "$BaseUrl/$Tag/$archiveName" -OutFile (Join-Path $TmpDir $archiveName)
    Invoke-MirrorDownload -Uri "$BaseUrl/$Tag/$checksumsName" -OutFile (Join-Path $TmpDir $checksumsName)
    return $Tag
}

# Downloads the archive and checksums from GitHub Releases into $TmpDir and
# returns the version.
function Save-GitHubRelease {
    param([string]$Tag, [string]$Arch, [string]$Repository, [string]$TmpDir)
    if ($Tag -eq "latest") {
        $Tag = Get-LatestTag -Repository $Repository
        if (-not $Tag -or -not $Tag.StartsWith("v")) {
            throw "Failed to resolve the latest version."
        }
    }
    $archiveName = Get-ArchiveName -Tag $Tag -Arch $Arch
    $checksumsName = Get-ChecksumsName -Tag $Tag
    $baseUrl = "https://github.com/$Repository/releases/download/$Tag"
    Write-Host "Downloading $archiveName from GitHub..."
    Invoke-WebRequest -Uri "$baseUrl/$archiveName" -OutFile (Join-Path $TmpDir $archiveName)
    Invoke-WebRequest -Uri "$baseUrl/$checksumsName" -OutFile (Join-Path $TmpDir $checksumsName)
    return $Tag
}

function Ensure-UserPathContains {
    param([string]$PathEntry)
    $current = [Environment]::GetEnvironmentVariable("Path", "User")
    if ([string]::IsNullOrWhiteSpace($current)) {
        [Environment]::SetEnvironmentVariable("Path", $PathEntry, "User")
        return $true
    }
    $parts = $current -split ";" | Where-Object { $_ -ne "" }
    if ($parts -contains $PathEntry) {
        return $false
    }
    $updated = ($parts + $PathEntry) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $updated, "User")
    return $true
}

function Install-Tollbit {
    param(
        [string]$Version = "latest",
        [string]$InstallDir = "$HOME\AppData\Local\Programs\tollbit\bin",
        [switch]$Force,
        [switch]$NoModifyPath,
        [switch]$PrintPathInstructions,
        [string]$Repo = "tollbit/cli",
        [string]$BaseUrl = "https://tollbit.com/.agents/cli"
    )

    $binaryName = "tollbit.exe"
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()) {
        "x64" { "amd64" }
        "arm64" { "arm64" }
        default { throw "Unsupported architecture: $_" }
    }

    if ($Version -ne "latest" -and -not $Version.StartsWith("v")) {
        throw "Version must start with 'v' (example: v0.0.1) or be 'latest'."
    }
    $BaseUrl = $BaseUrl.TrimEnd("/")
    $useMirror = ($Repo -eq $DefaultRepo) -and $BaseUrl

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("tollbit-install-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    try {
        $tag = $null
        if ($useMirror) {
            try {
                $tag = Save-MirrorRelease -Tag $Version -Arch $arch -BaseUrl $BaseUrl -TmpDir $tmpDir
            }
            catch {
                Write-Host "Could not download from ${BaseUrl}: $($_.Exception.Message)"
                Write-Host "Trying GitHub instead."
            }
        }
        if (-not $tag) {
            $tag = Save-GitHubRelease -Tag $Version -Arch $arch -Repository $Repo -TmpDir $tmpDir
        }

        $versionWithoutV = $tag.Substring(1)
        $archiveName = Get-ArchiveName -Tag $tag -Arch $arch
        $archivePath = Join-Path $tmpDir $archiveName
        $checksumsPath = Join-Path $tmpDir (Get-ChecksumsName -Tag $tag)
        $extractDir = Join-Path $tmpDir "extract"

        $checksumLine = Get-Content -Path $checksumsPath | Where-Object { $_ -match [regex]::Escape($archiveName) } | Select-Object -First 1
        if (-not $checksumLine) {
            throw "Checksum entry not found for $archiveName"
        }
        $expected = ($checksumLine -split "\s+")[0]
        $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($expected.ToLowerInvariant() -ne $actual) {
            throw "Checksum mismatch for $archiveName"
        }

        Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
        $extractedBin = Join-Path $extractDir $binaryName
        if (-not (Test-Path $extractedBin)) {
            throw "Archive did not contain $binaryName"
        }

        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        $destPath = Join-Path $InstallDir $binaryName
        if ((Test-Path $destPath) -and -not $Force) {
            $currentVersion = (& $destPath version 2>$null) -join ""
            if ($currentVersion -eq $versionWithoutV) {
                Write-Host "tollbit $currentVersion is already up to date at $destPath"
                return
            }
            throw "$destPath already exists. Re-run with -Force to overwrite."
        }

        $tmpDest = "$destPath.tmp"
        Copy-Item -Path $extractedBin -Destination $tmpDest -Force
        Move-Item -Path $tmpDest -Destination $destPath -Force

        # Record the install method so the CLI can show the right update command.
        Set-Content -Path (Join-Path $InstallDir ".tollbit-install-method") -Value "installer"

        $pathUpdated = $false
        if (-not $NoModifyPath) {
            $pathUpdated = Ensure-UserPathContains -PathEntry $InstallDir
        }

        Write-Host "Installed tollbit $versionWithoutV to $destPath"
        if ($pathUpdated) {
            Write-Host "Updated user PATH. Open a new shell session to use 'tollbit'."
        } elseif (-not $NoModifyPath) {
            Write-Host "User PATH already contains $InstallDir"
        }

        if ($PrintPathInstructions -or $NoModifyPath) {
            Write-Host "If 'tollbit' is not found, add this path to your user PATH:"
            Write-Host "  $InstallDir"
        }

        Write-Host "Verify install:"
        Write-Host "  tollbit version"
        Write-Host "Fallback full path:"
        Write-Host "  `"$destPath`" version"
    }
    finally {
        if (Test-Path $tmpDir) {
            Remove-Item -Path $tmpDir -Recurse -Force
        }
    }
}

Install-Tollbit -Version $Version -InstallDir $InstallDir -Force:$Force -NoModifyPath:$NoModifyPath -PrintPathInstructions:$PrintPathInstructions -Repo $Repo -BaseUrl $BaseUrl
