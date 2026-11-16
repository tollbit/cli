# Tests install.ps1 with Invoke-WebRequest and Invoke-RestMethod replaced by
# functions that serve two fixture folders: one for the tollbit.com mirror
# (https://mirror.test/.agents/cli/...) and one for GitHub Releases and its
# latest-release API. install.ps1 runs in a child scope, so it calls these
# functions instead of the cmdlets. Each test:
#   1. Builds both folders with release v1.2.3 and clears the request log.
#   2. Breaks the mirror or GitHub as the test needs, and runs install.ps1
#      into a fresh install directory.
#   3. Checks whether it succeeded, the installed binary and which hosts were
#      used.
#
# Nothing here touches the network or your PATH. Runs on any OS with pwsh.
#
# Usage:
#   pwsh -NoProfile -File scripts/install_test.ps1

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$InstallPs1 = Join-Path $PSScriptRoot "install.ps1"
$MirrorUrl = "https://mirror.test/.agents/cli"
$Root = Join-Path ([System.IO.Path]::GetTempPath()) ("tollbit-install-test-" + [guid]::NewGuid().ToString("N"))
$Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()) {
    "x64" { "amd64" }
    default { "arm64" }
}
$ArchiveName = "tollbit_1.2.3_windows_$Arch.zip"
$ChecksumsName = "tollbit_1.2.3_checksums.txt"

# All mutable test state. The fakes run inside install.ps1's scope, where
# $script: means install.ps1, so they reach this through normal lookup.
$State = @{
    Failures   = 0
    Requests   = [System.Collections.Generic.List[string]]::new()
    MirrorDown = $false
    Output     = ""
    Mirror     = ""
    GitHub     = ""
    InstallDir = ""
}

function Fail {
    param([string]$Message)
    Write-Host "    FAIL: $Message"
    $State.Failures++
}

function Assert-Equal {
    param($Want, $Got, [string]$What)
    if ($Want -ne $Got) {
        Fail "${What}: want '$Want', got '$Got'"
    }
}

function Assert-OutputContains {
    param([string]$Want)
    if (-not $State.Output.Contains($Want)) {
        Fail "output does not contain '$Want':`n$($State.Output)"
    }
}

function Count-Requests {
    param([string]$Prefix)
    return @($State.Requests | Where-Object { $_.StartsWith($Prefix) }).Count
}

function Assert-Installed {
    $dest = Join-Path $State.InstallDir "tollbit.exe"
    if (-not (Test-Path $dest)) {
        Fail "tollbit.exe was not installed"
        return
    }
    Assert-Equal (Get-Content -Raw (Join-Path $Root "build/tollbit.exe")) (Get-Content -Raw $dest) "installed binary"
}

# Maps a URL to its fixture file, or throws like a failed request.
function Resolve-FakeUrl {
    param([string]$Uri)
    $State.Requests.Add($Uri)
    if ($Uri.StartsWith("$MirrorUrl/")) {
        if ($State.MirrorDown) {
            throw "No such host is known. (mirror.test:443)"
        }
        $path = Join-Path $State.Mirror $Uri.Substring($MirrorUrl.Length + 1)
    }
    elseif ($Uri -match '^https://github\.com/[^/]+/[^/]+/releases/download/(.+)$') {
        $path = Join-Path $State.GitHub $Matches[1]
    }
    else {
        throw "fake: unexpected URL $Uri"
    }
    if (-not (Test-Path $path)) {
        throw "Response status code does not indicate success: 404 (Not Found)."
    }
    return $path
}

function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [int]$TimeoutSec, [switch]$UseBasicParsing)
    Copy-Item -Path (Resolve-FakeUrl -Uri $Uri) -Destination $OutFile
}

function Invoke-RestMethod {
    param([string]$Uri)
    $State.Requests.Add($Uri)
    if ($Uri -notmatch '^https://api\.github\.com/repos/[^/]+/[^/]+/releases/latest$') {
        throw "fake: unexpected URL $Uri"
    }
    return [pscustomobject]@{ tag_name = "v1.2.3" }
}

function Write-Release {
    param([string]$Dir)
    $releaseDir = Join-Path $Dir "v1.2.3"
    New-Item -ItemType Directory -Path $releaseDir -Force | Out-Null
    Copy-Item -Path (Join-Path $Root $ArchiveName) -Destination $releaseDir
    $hash = (Get-FileHash -Path (Join-Path $releaseDir $ArchiveName) -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -Path (Join-Path $releaseDir $ChecksumsName) -Value "$hash  $ArchiveName"
}

function Initialize-Fixture {
    $State.Mirror = Join-Path $Root "mirror"
    $State.GitHub = Join-Path $Root "github"
    $State.InstallDir = Join-Path $Root "install"
    foreach ($dir in @($State.Mirror, $State.GitHub, $State.InstallDir)) {
        if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
    }
    Write-Release -Dir $State.Mirror
    Set-Content -Path (Join-Path $State.Mirror "latest") -Value "version=v1.2.3"
    Write-Release -Dir $State.GitHub
    $State.Requests.Clear()
    $State.MirrorDown = $false
    $State.Output = ""
}

function Build-Archive {
    $build = Join-Path $Root "build"
    New-Item -ItemType Directory -Path $build -Force | Out-Null
    Set-Content -Path (Join-Path $build "tollbit.exe") -Value "fake tollbit 1.2.3"
    Compress-Archive -Path (Join-Path $build "tollbit.exe") -DestinationPath (Join-Path $Root $ArchiveName)
}

# Runs install.ps1 against the fixture with the given extra parameters. Sets
# Output; returns $true when it succeeded.
function Invoke-Install {
    param([hashtable]$Params = @{})
    $all = @{ BaseUrl = $MirrorUrl; InstallDir = $State.InstallDir; NoModifyPath = $true } + $Params
    try {
        $State.Output = (& $InstallPs1 @all 6>&1 | Out-String -Width 4096)
        return $true
    }
    catch {
        $State.Output += $_.Exception.Message
        return $false
    }
}

function Test-InstallsLatestFromTheMirror {
    if (-not (Invoke-Install)) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-Equal 3 (Count-Requests $MirrorUrl) "mirror requests"
    Assert-Equal 0 (Count-Requests "https://api.github.com") "GitHub API requests"
    Assert-Equal 0 (Count-Requests "https://github.com") "GitHub requests"
}

function Test-InstallsANamedVersionFromTheMirror {
    Remove-Item (Join-Path $State.Mirror "latest")
    if (-not (Invoke-Install @{ Version = "v1.2.3" })) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-Equal 0 (Count-Requests "https://github.com") "GitHub requests"
}

function Test-LatestIgnoresKeysItDoesNotKnow {
    Set-Content -Path (Join-Path $State.Mirror "latest") -Value @("channel=stable", "version=v1.2.3", "next=v9.9.9")
    if (-not (Invoke-Install)) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-Equal 0 (Count-Requests "https://github.com") "GitHub requests"
}

function Test-UnreachableMirrorFallsBackToGitHub {
    $State.MirrorDown = $true
    if (-not (Invoke-Install)) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-OutputContains "Trying GitHub instead."
    Assert-Equal 1 (Count-Requests "https://api.github.com") "GitHub API requests"
    Assert-Equal 2 (Count-Requests "https://github.com") "GitHub requests"
}

function Test-VersionMissingFromMirrorFallsBackToGitHub {
    Remove-Item -Recurse (Join-Path $State.Mirror "v1.2.3")
    if (-not (Invoke-Install @{ Version = "v1.2.3" })) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-Equal 0 (Count-Requests "https://api.github.com") "GitHub API requests"
    Assert-Equal 2 (Count-Requests "https://github.com") "GitHub requests"
}

function Test-LatestWithoutAVersionFallsBackToGitHub {
    Set-Content -Path (Join-Path $State.Mirror "latest") -Value "<html>blocked</html>"
    if (-not (Invoke-Install)) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-OutputContains "$MirrorUrl/latest has no version=v... line"
    Assert-Equal 1 (Count-Requests "https://api.github.com") "GitHub API requests"
}

function Test-MirrorChecksumMismatchFailsWithoutFallback {
    Add-Content -Path (Join-Path $State.Mirror "v1.2.3/$ArchiveName") -Value "tampered"
    if (Invoke-Install) { Fail "install succeeded" }
    Assert-OutputContains "Checksum mismatch for $ArchiveName"
    Assert-Equal 0 (Count-Requests "https://github.com") "GitHub requests"
    if (Test-Path (Join-Path $State.InstallDir "tollbit.exe")) { Fail "installed a binary" }
}

function Test-OtherRepoUsesOnlyGitHub {
    if (-not (Invoke-Install @{ Repo = "tollbit/cli-prereleases" })) { Fail "install failed: $($State.Output)" }
    Assert-Installed
    Assert-Equal 0 (Count-Requests $MirrorUrl) "mirror requests"
    Assert-Equal 2 (Count-Requests "https://github.com/tollbit/cli-prereleases/") "GitHub requests"
}

function Test-GitHubFailureAfterFallbackFails {
    $State.MirrorDown = $true
    Remove-Item (Join-Path $State.GitHub "v1.2.3/$ArchiveName")
    if (Invoke-Install) { Fail "install succeeded" }
    if (Test-Path (Join-Path $State.InstallDir "tollbit.exe")) { Fail "installed a binary" }
}

New-Item -ItemType Directory -Path $Root | Out-Null
try {
    Build-Archive
    $tests = Get-Command -CommandType Function -Name "Test-*" | Where-Object { $_.ScriptBlock.File -eq $PSCommandPath }
    foreach ($test in $tests) {
        Write-Host "--- $($test.Name)"
        Initialize-Fixture
        & $test.Name
    }
}
finally {
    Remove-Item -Recurse -Force $Root
}

if ($State.Failures -gt 0) {
    Write-Host "$($State.Failures) failure(s)"
    exit 1
}
Write-Host "all tests passed"
