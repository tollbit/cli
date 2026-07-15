[CmdletBinding()]
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$HOME\AppData\Local\Programs\tollbit\bin",
    [switch]$Force,
    [switch]$NoModifyPath,
    [switch]$PrintPathInstructions,
    [string]$Repo = "tollbit/cli"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-LatestTag {
    param([string]$Repository)
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest"
    return $release.tag_name
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
        [string]$Repo = "tollbit/cli"
    )

    $binaryName = "tollbit.exe"
    $os = "windows"
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()) {
        "x64" { "amd64" }
        "arm64" { "arm64" }
        default { throw "Unsupported architecture: $_" }
    }

    if ($Version -eq "latest") {
        $Version = Get-LatestTag -Repository $Repo
    }
    if (-not $Version.StartsWith("v")) {
        throw "Version must start with 'v' (example: v0.0.1) or be 'latest'."
    }

    $versionWithoutV = $Version.Substring(1)
    $assetBase = "tollbit_${versionWithoutV}_${os}_${arch}"
    $archiveName = "$assetBase.zip"
    $checksumsName = "tollbit_${versionWithoutV}_checksums.txt"
    $baseUrl = "https://github.com/$Repo/releases/download/$Version"
    $archiveUrl = "$baseUrl/$archiveName"
    $checksumsUrl = "$baseUrl/$checksumsName"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("tollbit-install-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    try {
        $archivePath = Join-Path $tmpDir $archiveName
        $checksumsPath = Join-Path $tmpDir $checksumsName
        $extractDir = Join-Path $tmpDir "extract"

        Write-Host "Downloading $archiveName..."
        Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath

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

Install-Tollbit -Version $Version -InstallDir $InstallDir -Force:$Force -NoModifyPath:$NoModifyPath -PrintPathInstructions:$PrintPathInstructions -Repo $Repo
