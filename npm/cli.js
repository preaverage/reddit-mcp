#!/usr/bin/env node
"use strict";

const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");

const SUPPORTED = ["linux-x64", "linux-arm64", "darwin-x64", "darwin-arm64", "win32-x64"];

function binaryPath() {
  const target = `${process.platform}-${process.arch}`;

  if (!SUPPORTED.includes(target)) {
    fail(
      `No build of reddit-mcp for ${target}.`,
      `Supported platforms: ${SUPPORTED.join(", ")}.`,
      "You can build from source instead: https://github.com/preaverage/reddit-mcp"
    );
  }

  const pkg = `${require("../package.json").name}-${target}`;
  const binary = process.platform === "win32" ? "reddit-mcp.exe" : "reddit-mcp";

  let manifest;
  try {
    manifest = require.resolve(`${pkg}/package.json`);
  } catch {
    fail(
      `The platform package ${pkg} is not installed.`,
      "This usually means npm skipped optional dependencies.",
      `Install it directly: npm install ${pkg}`
    );
  }

  const resolved = path.join(path.dirname(manifest), binary);

  if (!fs.existsSync(resolved)) {
    fail(`${pkg} is installed but ${binary} is missing from it.`);
  }

  ensureExecutable(resolved);

  return resolved;
}

function ensureExecutable(file) {
  if (process.platform === "win32") {
    return;
  }

  try {
    fs.accessSync(file, fs.constants.X_OK);
  } catch {
    try {
      fs.chmodSync(file, 0o755);
    } catch (err) {
      fail(`Cannot make ${file} executable: ${err.message}`);
    }
  }
}

function fail(...lines) {
  for (const line of lines) {
    console.error(line);
  }

  process.exit(1);
}

const FORWARDED_SIGNALS = ["SIGINT", "SIGTERM", "SIGHUP"];

const child = spawn(binaryPath(), process.argv.slice(2), { stdio: "inherit" });

for (const signal of FORWARDED_SIGNALS) {
  process.on(signal, () => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill(signal);
    }
  });
}

child.on("error", (err) => {
  fail(`Failed to start reddit-mcp: ${err.message}`);
});

child.on("exit", (code, signal) => {
  if (signal) {
    for (const forwarded of FORWARDED_SIGNALS) {
      process.removeAllListeners(forwarded);
    }

    process.kill(process.pid, signal);
    return;
  }

  process.exit(code === null ? 1 : code);
});
