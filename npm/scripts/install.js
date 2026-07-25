#!/usr/bin/env node
"use strict";

// Downloads the lynxor binary matching this package's own version from
// GitHub Releases (published by .goreleaser.yaml / release.yml), verifies it
// against the release's checksums.txt, and extracts it into ../.bin.
// bin/lynxor.js execs whatever ends up there -- this script's only job is
// to make sure the right binary is present and actually runs before install
// finishes, so failures surface at `npm install` time, not at first use.

const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const crypto = require("node:crypto");
const { spawnSync } = require("node:child_process");

const PLATFORMS = { linux: "linux", darwin: "darwin", win32: "windows" };
const ARCHES = { x64: "amd64", arm64: "arm64" };

function fail(message) {
  console.error(`lynxor: ${message}`);
  process.exit(1);
}

async function download(url) {
  const res = await fetch(url);
  if (!res.ok) {
    fail(`download failed (${res.status} ${res.statusText}): ${url}`);
  }
  return Buffer.from(await res.arrayBuffer());
}

async function main() {
  const platform = PLATFORMS[process.platform];
  const arch = ARCHES[process.arch];
  if (!platform || !arch) {
    fail(
      `unsupported platform/arch combination: ${process.platform}/${process.arch}. ` +
        `Supported: ${Object.keys(PLATFORMS).join(", ")} x ${Object.keys(ARCHES).join(", ")}. ` +
        `Build from source instead: https://github.com/xchebila/lynxor#install--build`,
    );
  }

  const pkg = require("../package.json");
  const version = process.env.LYNXOR_INSTALL_VERSION || pkg.version;
  const tag = `v${version}`;
  const ext = platform === "windows" ? "zip" : "tar.gz";
  const archiveName = `lynxor_${platform}_${arch}.${ext}`;
  const base = `https://github.com/xchebila/lynxor/releases/download/${tag}`;

  console.log(`lynxor: fetching ${archiveName} (${tag})...`);
  const [archive, checksums] = await Promise.all([
    download(`${base}/${archiveName}`),
    download(`${base}/checksums.txt`),
  ]);

  const expectedLine = checksums
    .toString("utf8")
    .split("\n")
    .find((line) => line.trim().endsWith(archiveName));
  if (!expectedLine) {
    fail(`${archiveName} not listed in checksums.txt for ${tag}`);
  }
  const expectedSum = expectedLine.trim().split(/\s+/)[0];
  const actualSum = crypto.createHash("sha256").update(archive).digest("hex");
  if (actualSum !== expectedSum) {
    fail(
      `checksum mismatch for ${archiveName}: expected ${expectedSum}, got ${actualSum}. ` +
        `Download may be corrupted or tampered with -- not installing.`,
    );
  }

  const destDir = path.join(__dirname, "..", ".bin");
  fs.rmSync(destDir, { recursive: true, force: true });
  fs.mkdirSync(destDir, { recursive: true });

  const archivePath = path.join(os.tmpdir(), `${archiveName}-${process.pid}`);
  fs.writeFileSync(archivePath, archive);
  try {
    // tar -xf handles both tar.gz and zip: GNU tar and macOS's bsdtar cover
    // the tar.gz case (linux/darwin), and Windows has shipped bsdtar as
    // tar.exe since Windows 10 1803 -- it extracts zip too. No npm
    // dependency needed for extraction on any of the three platforms.
    const result = spawnSync("tar", ["-xf", archivePath, "-C", destDir], {
      stdio: "inherit",
    });
    if (result.status !== 0) {
      fail(`extracting ${archiveName} failed (tar exited ${result.status})`);
    }
  } finally {
    fs.rmSync(archivePath, { force: true });
  }

  const binName = platform === "windows" ? "lynxor.exe" : "lynxor";
  const binPath = path.join(destDir, binName);
  if (!fs.existsSync(binPath)) {
    fail(`${binName} not found in ${archiveName} after extraction`);
  }
  if (platform !== "windows") {
    fs.chmodSync(binPath, 0o755);
  }

  const check = spawnSync(binPath, ["--version"], { encoding: "utf8" });
  if (check.status !== 0) {
    fail(`downloaded binary failed to run: ${check.stderr || check.error}`);
  }
  console.log(`lynxor: installed ${check.stdout.trim()}`);
}

main().catch((err) => fail(err.stack || String(err)));
