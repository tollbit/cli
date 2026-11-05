# Diagnose a failed tollbit CLI install

Work out where installing the tollbit CLI breaks on this machine, then write a short report the user sends to TollBit support. On a managed work computer some fixes need the user's own IT administrators; TollBit support works with them when that's the case. This covers installing only. If `tollbit version` already prints a version, the install worked: say so, and stop. If `tollbit` isn't found but step 5 finds a binary that runs, an earlier install worked and only PATH is missing: run every step, and report it as a PATH problem.

Rules:

- **Diagnose, don't work around.** Don't install from any source other than those listed below, turn off TLS verification (except for the read-only issuer lookup in step 3), change system or security settings, change npm or shell configuration, or ask for an administrator password. Suggest such changes in the report instead.
- **Change nothing on the machine.** The checks only read, except for test files they remove at once (in the install folder on Windows, and in a temporary folder) and step 5's download into a temporary folder it deletes.
- **Run every step**, even after one fails. The steps check different things, and the report needs all of them.
- Keep the exact error text of every failing command. Before you show anything, replace passwords and tokens (for example the `user:pass` in a proxy URL) with `***`.
- Use the commands for the shell you have: bash or zsh on macOS and Linux, PowerShell on Windows. In Windows PowerShell 5.1, `curl` means `Invoke-WebRequest`; write `curl.exe` for real curl.

## 1. Environment

Find out, and note for the report:

- **Where you run.** One of these:
  - Claude in the desktop or web app, chat or Cowork: a temporary Linux sandbox whose network goes through a proxy (`HTTPS_PROXY` is set), limited by the organization's allowlist.
  - Claude Code, on the user's computer.
  - Codex, whose sandbox often has no network at all.
  - A terminal the user runs commands in themselves.
- **What failed.** Which install method was tried (script, npm, the tollbit-install-claude skill) and its exact error. Find it earlier in the conversation, or ask the user to paste it. If they can't, and the user agrees, rerun that install once to capture it: for npm, `npm install -g @tollbit/tollbit-cli --foreground-scripts` shows the install step's own error. If what failed is adding the TollBit plugin itself in the Claude app (for example, no option to add a marketplace or upload a plugin), that comes from the organization's Claude settings, not this machine or the plugin's files: user-created skills turned off, Skills or Cowork turned off, or a limit on which repositories can be added. Note it in the report for an organization admin, and check the CLI install anyway.
- **OS, CPU architecture and shell.**

  ```bash
  uname -srm; echo "shell: $SHELL"
  for c in curl tar shasum sha256sum node npm; do printf '%s: ' "$c"; command -v "$c" || echo missing; done
  node --version; npm --version
  ```

  ```powershell
  [Environment]::OSVersion.VersionString
  [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture
  "PowerShell $($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition)"
  "Language mode: $($ExecutionContext.SessionState.LanguageMode)"
  Get-ExecutionPolicy -List
  foreach ($c in 'node', 'npm', 'curl.exe') { "${c}: $((Get-Command $c -ErrorAction SilentlyContinue).Source)" }
  node --version; npm --version
  ```

What the results mean:

- Only `amd64`/`x86_64` and `arm64`/`aarch64` are supported, on macOS, Linux and Windows. The npm package needs Node 18 or later.
- **Language mode `ConstrainedLanguage`** (set by AppLocker or WDAC) breaks `irm ... | iex`, the PowerShell installer.
- **Execution policy** applies only to `.ps1` files saved to disk. `Restricted` or `AllSigned` blocks running a saved `install.ps1`, but not `irm ... | iex`. It also blocks `npm.ps1`, so `npm` can fail in PowerShell while `npm.cmd` works.

## 2. Proxy settings

```bash
env | grep -i -E '^(https?|all|no)_proxy=' | sed -E 's#//[^@/]*@#//***@#'
echo "NODE_EXTRA_CA_CERTS=$NODE_EXTRA_CA_CERTS NODE_USE_ENV_PROXY=$NODE_USE_ENV_PROXY"
for k in proxy https-proxy noproxy cafile strict-ssl ignore-scripts registry prefix; do echo "npm $k = $(npm config get $k 2>&1)"; done | sed -E 's#//[^@/]*@#//***@#'
[ "$(uname)" = Darwin ] && { scutil --proxy | grep -E 'HTTPS(Enable|Proxy|Port)|ProxyAutoConfig(Enable|URLString)|ProxyAutoDiscoveryEnable' || echo "macOS system proxy: none"; }
```

```powershell
Get-ChildItem Env: | Where-Object Name -Match '^(https?|all|no)_proxy$|^NODE_EXTRA_CA_CERTS$|^NODE_USE_ENV_PROXY$' | ForEach-Object { "$($_.Name)=$($_.Value -replace '//[^@/]*@', '//***@')" }
foreach ($k in 'proxy', 'https-proxy', 'noproxy', 'cafile', 'strict-ssl', 'ignore-scripts', 'registry', 'prefix') { "npm $k = $(npm.cmd config get $k 2>&1)" -replace '//[^@/]*@', '//***@' }
netsh winhttp show proxy
Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' | Select-Object ProxyEnable, ProxyServer, AutoConfigURL
```

What the results mean:

- Each install method finds the proxy in a different place:
  - `install.sh` (curl) reads the `HTTPS_PROXY` environment variable.
  - `install.ps1` (`Invoke-WebRequest`) uses the Windows proxy in Internet Settings, and also `HTTPS_PROXY` in PowerShell 7.
  - npm fetches the package with its own `proxy`/`https-proxy` settings. The package's install step (`npm/install.js`) then downloads the binary from GitHub using only the `HTTPS_PROXY`/`NO_PROXY` environment variables.
- **A proxy set in npm but not in `HTTPS_PROXY`:** npm gets the package, then the binary download goes direct and fails with `no data for 60s` or a connection error.
- **A system proxy is set but `HTTPS_PROXY` isn't** (macOS `HTTPSEnable : 1` from `scutil --proxy`, or Windows `ProxyEnable` 1): browsers use it, but curl, Node and the CLI don't. Setting `HTTPS_PROXY` to the same address fixes the install.
- **Only a PAC file is set** (Windows `AutoConfigURL`, macOS `ProxyAutoConfigEnable : 1` or `ProxyAutoDiscoveryEnable : 1`): curl and Node can't use a PAC file. The user needs the actual proxy address from IT.
- **`ignore-scripts = true`:** npm never runs the package's install step, so the binary is never downloaded.
- Behind a proxy that inspects TLS (step 3), Node needs `NODE_EXTRA_CA_CERTS` pointing at the corporate root certificate. npm's `cafile` doesn't apply to the install step.

## 3. Hosts

These are the hosts installing needs:

| Host | Needed by | Probe URL |
|---|---|---|
| `raw.githubusercontent.com` | `install.sh`, `install.ps1` | https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh |
| `api.github.com` | the scripts, to find the latest version (not needed when a version is given) | https://api.github.com/repos/tollbit/cli/releases/latest |
| `github.com` | every method: release downloads, and Claude Code's `/plugin marketplace add` | https://github.com/tollbit/cli/releases |
| `release-assets.githubusercontent.com` | every method: GitHub redirects release downloads here | https://github.com/tollbit/cli/releases/download/v0.5.1/tollbit_0.5.1_checksums.txt (follows the redirect) |
| `registry.npmjs.org` | npm | https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest |

`release-assets.githubusercontent.com` has no probe of its own: rate it by the release download line, after the redirect. If `github.com` passes and the release download fails, `release-assets.githubusercontent.com` is the blocked host.

```bash
for url in \
  https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh \
  https://api.github.com/repos/tollbit/cli/releases/latest \
  https://github.com/tollbit/cli/releases \
  https://github.com/tollbit/cli/releases/download/v0.5.1/tollbit_0.5.1_checksums.txt \
  https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest
do
  echo "== $url"
  curl -sSL -o /dev/null --max-time 20 -v -w 'HTTP %{http_code} in %{time_total}s\n' "$url" 2>&1 \
    | grep -E '^(HTTP |curl: |\* +(issuer|subject):|< HTTP/|< (x-deny-reason|x-squid-error|x-ratelimit-remaining):)|CONNECT tunnel'
done
```

```powershell
$urls = @(
  'https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh',
  'https://api.github.com/repos/tollbit/cli/releases/latest',
  'https://github.com/tollbit/cli/releases',
  'https://github.com/tollbit/cli/releases/download/v0.5.1/tollbit_0.5.1_checksums.txt',
  'https://registry.npmjs.org/@tollbit%2ftollbit-cli/latest'
)
foreach ($u in $urls) {
  try {
    $r = Invoke-WebRequest -Uri $u -UseBasicParsing -TimeoutSec 20 -ErrorAction Stop
    "OK   $($r.StatusCode) $u"
  } catch {
    $code = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { '-' }
    "FAIL $code $u`n     $($_.Exception.GetType().Name): $($_.Exception.Message) $($_.Exception.InnerException.Message)"
  }
}
```

On Windows, also run `curl.exe -sSIv --max-time 20 <url>` for each failing URL to see the certificate issuer. curl.exe ignores the Windows proxy, so add `--proxy <ProxyServer from step 2>` when one is set.

When a host fails with a certificate error, read the certificate's issuer with `curl -sSkLv -o /dev/null --max-time 20 <url> 2>&1 | grep -E '^\* +(subject|issuer):'`. `-k` skips verification so the issuer can be printed; the response is discarded, and nothing downloaded this way is used. Each `subject:` line names the host (`CN=...`), and the `issuer:` line after it is who signed that host's certificate.

Name each host's result with one of these (`tollbit doctor` uses the same names):

| Result | What you see |
|---|---|
| **ok** | HTTP 200 (`raw.githubusercontent.com` may be 200 or 304). |
| **dns** | `curl: (6) Could not resolve host`; PowerShell: `No such host is known` or `The remote name could not be resolved`. The network has no DNS for the host, or blocks it there. |
| **timeout** | `curl: (28)`; PowerShell: `The operation has timed out`. Usually a firewall dropping traffic, or a proxy that's needed but not configured. |
| **connect** | `curl: (7) Failed to connect`, or a connection reset (`curl: (35)` or `(56)`) right after connecting. |
| **tls_intercepted** | `curl: (60) SSL certificate problem` (`unable to get local issuer certificate`, `self-signed certificate in certificate chain`); PowerShell: `Could not establish trust relationship`; Node: `UNABLE_TO_GET_ISSUER_CERT_LOCALLY` or `SELF_SIGNED_CERT_IN_CHAIN`. Or the request passes but the `issuer:` line names a corporate CA or a security product (Zscaler, Netskope, Palo Alto, Forcepoint, Blue Coat, or the company's name) instead of a public CA such as DigiCert or Sectigo. Windows curl.exe behind such a proxy often fails with `CRYPT_E_NO_REVOCATION_CHECK`. Write down the issuer. |
| **tls_other** | Any other TLS failure, such as an expired certificate or a failed handshake. |
| **proxy_rejected** | `CONNECT tunnel failed, response 403` (blocked) or `407` (the proxy wants a login), or a 403 or "blocked" page that names the proxy or the company. In Claude's app sandbox this means the organization's allowlist blocks the host. |
| **`http_<code>`** | Any other HTTP error from the host itself, for example `http_404`. On `api.github.com`, a 403 or 429 with `x-ratelimit-remaining: 0` is GitHub's rate limit (many users share one office IP address), not a block: write `http_403 (rate limit)`. Giving the scripts a version (`--version` or `-Version`) avoids the call. |

If every host fails the same way and you are in an agent sandbox (Codex, or Claude Code with sandboxing turned on), your sandbox may be what blocks the network. Ask the user to allow network access for the command and run it again before blaming their network.

## 4. Write access and PATH, without admin

```bash
d="$HOME/.local/bin"
e="$d"; while [ ! -e "$e" ]; do e="$(dirname "$e")"; done  # the installer creates what's missing
if [ -w "$e" ]; then echo "install dir writable: $d (checked $e)"; else echo "install dir NOT writable: $d (checked $e)"; fi
case ":$PATH:" in *":$d:"*) echo "$d is on PATH";; *) echo "$d is NOT on PATH";; esac
for f in ~/.zshrc ~/.bashrc ~/.profile; do [ -e "$f" ] && { [ -w "$f" ] && echo "$f writable" || echo "$f NOT writable"; }; done
t="$(mktemp -d)" && echo "temp dir writable: $t" && rmdir "$t"
echo "elevated (root): $([ "$(id -u)" = 0 ] && echo yes || echo no)"
p="$(npm prefix -g 2>/dev/null)"
if [ -z "$p" ]; then echo "npm not available"
elif [ -w "$p/lib/node_modules" ] || { [ ! -e "$p/lib/node_modules" ] && [ -w "$p" ]; }; then echo "npm global prefix writable: $p"
else echo "npm global prefix NOT writable: $p (npm -g needs sudo)"; fi
```

```powershell
$d = "$env:LOCALAPPDATA\Programs\tollbit\bin"
$e = $d; while (-not (Test-Path $e)) { $e = Split-Path $e }  # the installer creates what's missing
try { Set-Content "$e\.tollbit-write-test" x -ErrorAction Stop; Remove-Item "$e\.tollbit-write-test"; "install dir writable: $d (checked $e)" } catch { "install dir NOT writable: $d (checked $e: $($_.Exception.Message))" }
"on user PATH: $(([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') -contains $d)"
# Opens the key with write access without writing to it.
try { $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true); $k.Close(); 'user PATH writable' } catch { "user PATH NOT writable: $($_.Exception.Message)" }
"temp dir: $env:TEMP"
$p = npm.cmd prefix -g 2>$null
if (-not $p) { 'npm not available' }
else { try { Set-Content "$p\.tollbit-write-test" x -ErrorAction Stop; Remove-Item "$p\.tollbit-write-test"; "npm global prefix writable: $p" } catch { "npm global prefix NOT writable: $p (npm -g needs admin)" } }
"elevated: $(([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole('Administrators'))"
```

What the results mean:

- The scripts install to `~/.local/bin` or `%LOCALAPPDATA%\Programs\tollbit\bin` and add it to the user's PATH. Neither needs admin.
- **npm prefix not writable** (for example under `C:\Program Files\nodejs` or `/usr/local`): `npm install -g` needs admin. On Windows it may wait on an admin prompt the agent can't see, which looks like a hang. The fix is the script installer, or a user-owned npm prefix. Suggest these; don't apply them.
- **Install directory not on PATH:** the install may have worked. Open a new shell (or run `hash -r`) and check again, or run `tollbit` by its full path.

## 5. Does the binary run

Look for a `tollbit` binary from earlier attempts:

```bash
command -v tollbit
for f in ~/.local/bin/tollbit "$(npm prefix -g 2>/dev/null)/lib/node_modules/@tollbit/tollbit-cli/tollbit"; do [ ! -e "$f" ] || ls -l "$f"; done
```

```powershell
Get-Command tollbit -All -ErrorAction SilentlyContinue | Select-Object Source
$p = npm.cmd prefix -g
Get-Item "$env:LOCALAPPDATA\Programs\tollbit\bin\tollbit.exe", "$p\node_modules\@tollbit\tollbit-cli\tollbit.exe" -ErrorAction SilentlyContinue
```

Run each binary you find by its full path, with `version`.

If none exists and the release download was **ok** in step 3, test a fresh copy in a temporary directory. This checks the binary on its own, apart from any installer. The `<os>_<arch>` part is `darwin_arm64`, `linux_amd64`, `windows_amd64` and so on.

```bash
t="$(mktemp -d)"; cd "$t"
base=https://github.com/tollbit/cli/releases/download/v0.5.1
curl -fsSLO "$base/tollbit_0.5.1_checksums.txt"
a="$(grep -o 'tollbit_[^ ]*_<os>_<arch>.tar.gz' tollbit_*_checksums.txt)"
curl -fsSLO "$base/$a" && grep " $a\$" tollbit_*_checksums.txt | (shasum -a 256 -c 2>/dev/null || sha256sum -c) \
  && tar -xzf "$a" && ./tollbit version
cd - >/dev/null; rm -rf "$t"
```

```powershell
$t = Join-Path $env:TEMP "tollbit-test-$([guid]::NewGuid().ToString('N'))"; New-Item -ItemType Directory $t | Out-Null
$base = 'https://github.com/tollbit/cli/releases/download/v0.5.1'
Invoke-WebRequest "$base/tollbit_0.5.1_checksums.txt" -OutFile "$t\sums.txt" -UseBasicParsing
$line = Select-String -Path "$t\sums.txt" -Pattern '_windows_<arch>\.zip$' | Select-Object -First 1
$sum, $a = $line.Line -split '\s+'
Invoke-WebRequest "$base/$a" -OutFile "$t\$a" -UseBasicParsing
if ((Get-FileHash "$t\$a" -Algorithm SHA256).Hash -eq $sum) { Expand-Archive "$t\$a" "$t\x"; & "$t\x\tollbit.exe" version } else { "checksum mismatch for ${a}: not running it" }
Remove-Item -Recurse -Force $t
```

What the results mean:

- **It prints a version:** the binary works. A failed install is then about the download, write access or PATH. If it isn't on PATH, an earlier install worked and only PATH is missing. An older version is fine for this check; note it in the report.
- **`tollbit` exits with code 1 and prints nothing, after an npm install:** the package is there but its binary isn't, so the install step was skipped (`ignore-scripts`) or failed. Rerun `npm install -g @tollbit/tollbit-cli --foreground-scripts` and keep its output.
- **`exec format error` or `cannot execute binary file`:** the binary is for the wrong CPU (`file <path>` shows which). On Windows on ARM, x64 PowerShell or Node picks the amd64 build.
- **macOS, `killed: 9`, or "cannot be opened because the developer cannot be verified":** Gatekeeper. `xattr -l <path>` shows whether `com.apple.quarantine` is set.
- **Linux, `Permission denied` although the file is executable:** the directory is mounted `noexec`. `findmnt -T <path> -o OPTIONS` shows it.
- **Windows, `This program is blocked by group policy` or `Access is denied`:** AppLocker, WDAC or a software restriction policy blocks programs in user folders.
- **Windows, `Operation did not complete successfully because the file contains a virus`, or the file disappears after the download:** antivirus quarantined it.
- **Windows SmartScreen prompt:** the file has a `Zone.Identifier` stream (`Get-Item <path> -Stream Zone.Identifier`), usually because it came through a browser.

## 6. Report

Give the user this report in a code block. Fill in every line, writing `not checked` where a step didn't apply. Ask them to send it to TollBit support, and to look it over first: it contains folder paths, which may include their username.

```text
TollBit CLI install diagnostic
Date:
Run by: <Claude app sandbox | Claude Code | Codex | terminal>, <agent and model if known>
OS / arch / shell:
PowerShell language mode / execution policy:      (Windows only)
Install tried: <method>; error: <exact text>

Proxy:  env <HTTPS_PROXY / none>; npm <proxy / none>; system <macOS or Windows proxy / PAC / none>   (credentials removed)
TLS inspection: <none seen | issuer "...">; NODE_EXTRA_CA_CERTS <set / not set>

Hosts:
  raw.githubusercontent.com             <ok | dns | connect | timeout | proxy_rejected | tls_intercepted | tls_other | http_<code>>  <detail>
  api.github.com                        <...>
  github.com                            <...>
  release-assets.githubusercontent.com  <...>
  registry.npmjs.org                    <...>

Write access: install dir <yes/no>; on PATH <yes/no>; npm prefix <path>, writable <yes/no>; elevated <yes/no>
Binary: <found at ... | none>; runs <yes: version | no: error>

Likely cause:
Suggested next step:
Fix likely needs: <you | your IT administrators | your Claude organization admin | TollBit>
```

For **Likely cause**, name the first thing that breaks the install method the user tried. Common ones:

| Finding | Suggested next step | Who |
|---|---|---|
| `raw.githubusercontent.com` blocked, GitHub release downloads OK | Install with npm, or manually from https://github.com/tollbit/cli/releases (download the archive, check its SHA-256, put the binary on PATH). | you |
| `github.com` or `release-assets.githubusercontent.com` blocked | Every install method downloads from GitHub Releases. Allow both hosts. | IT administrators |
| npm registry OK, GitHub download fails in npm's install step | Set `HTTPS_PROXY` to the same proxy npm uses, then install again; or allow the GitHub hosts. | you, or IT administrators |
| TLS inspection | IT provides the corporate root certificate (a PEM file); for npm, set `NODE_EXTRA_CA_CERTS` to it. Don't turn off verification. | IT administrators, then you |
| npm prefix needs admin | Use the script installer, or set a user-owned npm prefix. If Node is installed and managed by the company, IT may need to do this. | you, or IT administrators |
| PowerShell `ConstrainedLanguage` | The PowerShell installer can't run. Install manually from GitHub Releases, if policy allows running it. | IT administrators |
| Binary blocked by AppLocker, WDAC or antivirus | Allow `tollbit.exe` from https://github.com/tollbit/cli/releases. Each release can be verified with `gh attestation verify <archive> --repo tollbit/cli`. | IT administrators |
| Claude app sandbox, proxy_rejected on GitHub or npm | Allow package managers and GitHub under **Organization settings → Capabilities → Code execution**; then start a new session. | Claude organization admin |
| Codex or sandboxed Claude Code, every host fails the same way | Allow network access for the agent, then try again. | you |
| Checksum mismatch, a binary that crashes, or no cause found | TollBit support looks into it. | TollBit |

Fill in **Fix likely needs** from the **Who** column. On a managed work computer that is often the user's IT administrators, since only they can change the network, the proxy or what programs may run. Tell the user so, and that TollBit support will bring in their IT administrators where needed. The user sends the report to TollBit support only, not to their IT administrators.
