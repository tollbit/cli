#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const path = require("path");

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, "tollbit" + ext);

try {
  execFileSync(bin, process.argv.slice(2), {
    stdio: "inherit",
    env: { ...process.env, TOLLBIT_INSTALL_METHOD: "npm" }
  });
} catch (error) {
  process.exitCode = error.status || 1;
}
