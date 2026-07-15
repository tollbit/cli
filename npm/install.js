"use strict";

const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
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

function fail(message) {
  throw new Error(message);
}

function download(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          download(res.headers.location).then(resolve).catch(reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`Download failed (${res.statusCode}) for ${url}`));
          return;
        }
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
      })
      .on("error", reject);
  });
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

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
