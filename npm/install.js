"use strict";

const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const https = require("https");
const net = require("net");
const os = require("os");
const path = require("path");
const tls = require("tls");
const { execFileSync } = require("child_process");

const repo = process.env.TOLLBIT_NPM_REPO || "tollbit/cli";
const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, "package.json"), "utf8"));
const version = process.env.TOLLBIT_VERSION_OVERRIDE || `v${pkg.version}`;

const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows"
};

const archMap = {
  x64: "amd64",
  arm64: "arm64"
};

const downloadTimeoutMs = 60 * 1000;
const maxRedirects = 5;

function fail(message) {
  throw new Error(message);
}

function envAny(env, names) {
  for (const name of names) {
    if (env[name]) {
      return env[name];
    }
  }
  return "";
}

function stripBrackets(host) {
  return host.startsWith("[") && host.endsWith("]") ? host.slice(1, -1) : host;
}

function defaultPort(url) {
  return url.protocol === "http:" ? "80" : "443";
}

// noProxyMatches follows Go's net/http rules, so the installer and the CLI
// agree: "*" matches everything; "example.com" matches it and its
// subdomains; ".example.com" and "*.example.com" match subdomains only; IPs
// and CIDR ranges match addresses; an entry with ":port" matches that port only.
function noProxyMatches(host, port, noProxy) {
  for (let entry of noProxy.split(",")) {
    entry = entry.trim().toLowerCase();
    if (!entry) {
      continue;
    }
    if (entry === "*") {
      return true;
    }
    if (entry.includes("/")) {
      const [base, bits] = entry.split("/");
      const family = net.isIP(base);
      // Like Go, skip a malformed CIDR entry instead of failing the install.
      const validPrefix = /^\d+$/.test(bits) && Number(bits) <= (family === 6 ? 128 : 32);
      if (family && validPrefix && net.isIP(host) === family) {
        const list = new net.BlockList();
        list.addSubnet(base, Number(bits), family === 6 ? "ipv6" : "ipv4");
        if (list.check(host, family === 6 ? "ipv6" : "ipv4")) {
          return true;
        }
      }
      continue;
    }

    let entryHost = entry;
    let entryPort = "";
    const bracketed = entry.match(/^\[(.+)\](?::(\d+))?$/);
    if (bracketed) {
      entryHost = bracketed[1];
      entryPort = bracketed[2] || "";
    } else if (!net.isIP(entry)) {
      const withPort = entry.match(/^(.+):(\d+)$/);
      if (withPort) {
        entryHost = withPort[1];
        entryPort = withPort[2];
      }
    }
    if (entryPort && entryPort !== port) {
      continue;
    }
    if (net.isIP(entryHost)) {
      if (net.isIP(host) && entryHost === host) {
        return true;
      }
      continue;
    }
    if (entryHost.startsWith("*.")) {
      entryHost = entryHost.slice(1);
    }
    if (entryHost.startsWith(".")) {
      if (host.endsWith(entryHost)) {
        return true;
      }
    } else if (host === entryHost || host.endsWith(`.${entryHost}`)) {
      return true;
    }
  }
  return false;
}

// proxyForUrl returns the proxy to use for url from HTTPS_PROXY / HTTP_PROXY
// and NO_PROXY (either case, upper case first), or null for a direct
// connection. Like Go, it never proxies localhost or loopback addresses.
function proxyForUrl(url, env = process.env) {
  const target = new URL(url);
  const host = stripBrackets(target.hostname).toLowerCase();
  if (host === "localhost" || (net.isIP(host) && isLoopback(host))) {
    return null;
  }
  const value =
    target.protocol === "http:" ? envAny(env, ["HTTP_PROXY", "http_proxy"]) : envAny(env, ["HTTPS_PROXY", "https_proxy"]);
  if (!value) {
    return null;
  }
  const noProxy = envAny(env, ["NO_PROXY", "no_proxy"]);
  if (noProxyMatches(host, target.port || defaultPort(target), noProxy)) {
    return null;
  }
  const proxy = new URL(/^[a-z][a-z0-9+.-]*:\/\//i.test(value) ? value : `http://${value}`);
  if (proxy.protocol !== "http:" && proxy.protocol !== "https:") {
    fail(`Unsupported proxy scheme ${proxy.protocol} (only http and https proxies are supported)`);
  }
  return proxy;
}

function isLoopback(ip) {
  const list = new net.BlockList();
  list.addSubnet("127.0.0.0", 8, "ipv4");
  list.addAddress("::1", "ipv6");
  return list.check(ip, net.isIP(ip) === 6 ? "ipv6" : "ipv4");
}

// describeProxy names a proxy without its credentials.
function describeProxy(proxy) {
  return `${proxy.protocol}//${proxy.host}`;
}

// openTunnel asks the proxy to open a connection to target (HTTP CONNECT) and
// returns the connected socket.
function openTunnel(proxy, target, timeoutMs) {
  return new Promise((resolve, reject) => {
    const authority = `${target.hostname}:${target.port || defaultPort(target)}`;
    const headers = { host: authority };
    if (proxy.username || proxy.password) {
      const credentials = `${decodeURIComponent(proxy.username)}:${decodeURIComponent(proxy.password)}`;
      headers["proxy-authorization"] = `Basic ${Buffer.from(credentials).toString("base64")}`;
    }
    const client = proxy.protocol === "https:" ? https : http;
    const req = client.request({
      host: stripBrackets(proxy.hostname),
      port: proxy.port || defaultPort(proxy),
      method: "CONNECT",
      path: authority,
      headers,
      agent: false
    });
    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error(`Proxy ${describeProxy(proxy)} did not respond within ${timeoutMs / 1000}s`));
    });
    req.on("connect", (res, socket) => {
      if (res.statusCode !== 200) {
        socket.destroy();
        reject(new Error(`Proxy ${describeProxy(proxy)} refused to connect to ${authority} (${res.statusCode})`));
        return;
      }
      socket.setTimeout(0);
      resolve(socket);
    });
    req.on("error", reject);
    req.end();
  });
}

// download fetches url, through the proxy from the environment when one
// applies, following redirects. It fails when no data arrives for timeoutMs.
async function download(url, options = {}) {
  const { env = process.env, timeoutMs = downloadTimeoutMs, redirects = 0 } = options;
  const target = new URL(url);
  const client = target.protocol === "http:" ? http : https;
  let requestOptions = {};
  try {
    const proxy = proxyForUrl(target, env);
    if (proxy) {
      const socket = await openTunnel(proxy, target, timeoutMs);
      const host = stripBrackets(target.hostname);
      requestOptions = {
        // Without an agent Node assumes port 80 for the Host header, so name
        // the scheme's default port.
        defaultPort: Number(defaultPort(target)),
        createConnection: () =>
          target.protocol === "https:" ? tls.connect({ socket, servername: net.isIP(host) ? undefined : host }) : socket
      };
    }
  } catch (error) {
    throw new Error(`Download failed for ${url}: ${error.message}`);
  }

  const result = await new Promise((resolve, reject) => {
    let timedOut = false;
    const failWith = (error) => {
      const reason = timedOut ? `no data for ${timeoutMs / 1000}s` : error.message;
      reject(new Error(`Download failed for ${url}: ${reason}`));
    };
    const req = client.get(target, requestOptions, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        resolve({ redirect: new URL(res.headers.location, target).toString() });
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`Download failed (${res.statusCode}) for ${url}`));
        return;
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("error", failWith);
      res.on("close", () => {
        if (res.complete) {
          resolve({ data: Buffer.concat(chunks) });
        } else {
          failWith(new Error("connection closed before the download finished"));
        }
      });
    });
    req.setTimeout(timeoutMs, () => {
      timedOut = true;
      req.destroy(new Error("timed out"));
    });
    req.on("error", failWith);
  });

  if (result.redirect) {
    if (redirects >= maxRedirects) {
      fail(`Download failed for ${url}: too many redirects`);
    }
    return download(result.redirect, { env, timeoutMs, redirects: redirects + 1 });
  }
  return result.data;
}

function checksumForAsset(checksums, assetName) {
  const line = checksums
    .toString("utf8")
    .split(/\r?\n/)
    .find((entry) => entry.trim().endsWith(` ${assetName}`));
  if (!line) {
    fail(`Checksum entry not found for ${assetName}`);
  }
  return line.trim().split(/\s+/)[0];
}

function verifyChecksum(data, expected) {
  const actual = crypto.createHash("sha256").update(data).digest("hex");
  if (actual !== expected) {
    fail(`Checksum mismatch. Expected ${expected}, got ${actual}`);
  }
}

function extractArchive(archivePath, outputDir, archiveName) {
  if (archiveName.endsWith(".tar.gz")) {
    execFileSync("tar", ["-xzf", archivePath, "-C", outputDir], { stdio: "inherit" });
    return;
  }

  if (archiveName.endsWith(".zip")) {
    if (process.platform === "win32") {
      const command = `Expand-Archive -Path "${archivePath}" -DestinationPath "${outputDir}" -Force`;
      execFileSync("powershell.exe", ["-NoProfile", "-Command", command], { stdio: "inherit" });
      return;
    }
    execFileSync("unzip", ["-o", archivePath, "-d", outputDir], { stdio: "inherit" });
    return;
  }

  fail(`Unsupported archive format: ${archiveName}`);
}

async function main() {
  const platform = platformMap[process.platform];
  const arch = archMap[process.arch];
  if (!platform || !arch) {
    fail(`Unsupported platform/arch: ${process.platform}/${process.arch}`);
  }

  const versionWithoutV = version.replace(/^v/, "");
  const assetBase = `tollbit_${versionWithoutV}_${platform}_${arch}`;
  const archiveName = platform === "windows" ? `${assetBase}.zip` : `${assetBase}.tar.gz`;
  const checksumsName = `tollbit_${versionWithoutV}_checksums.txt`;
  const baseUrl = `https://github.com/${repo}/releases/download/${version}`;
  const archiveUrl = `${baseUrl}/${archiveName}`;
  const checksumsUrl = `${baseUrl}/${checksumsName}`;

  const [checksums, archive] = await Promise.all([download(checksumsUrl), download(archiveUrl)]);
  const expected = checksumForAsset(checksums, archiveName);
  verifyChecksum(archive, expected);

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "tollbit-npm-"));
  try {
    const archivePath = path.join(tmpDir, archiveName);
    const extractDir = path.join(tmpDir, "extract");
    fs.mkdirSync(extractDir, { recursive: true });
    fs.writeFileSync(archivePath, archive);

    extractArchive(archivePath, extractDir, archiveName);

    const ext = process.platform === "win32" ? ".exe" : "";
    const src = path.join(extractDir, `tollbit${ext}`);
    const dest = path.join(__dirname, `tollbit${ext}`);
    if (!fs.existsSync(src)) {
      fail(`Extracted archive did not contain ${path.basename(dest)}`);
    }

    fs.copyFileSync(src, dest);
    if (process.platform !== "win32") {
      fs.chmodSync(dest, 0o755);
    }
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}

module.exports = { download, proxyForUrl };
