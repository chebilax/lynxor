#!/usr/bin/env node
"use strict";

const path = require("node:path");
const { spawnSync } = require("node:child_process");

const binName = process.platform === "win32" ? "reposcan.exe" : "reposcan";
const binPath = path.join(__dirname, "..", ".bin", binName);

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(
    `reposcan: failed to launch ${binPath}: ${result.error.message}\n` +
      `Try reinstalling: npm install reposcan`,
  );
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
