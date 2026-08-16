#!/usr/bin/env node
// Local Windows/China: GitHub release-asset downloads often die with EOF.
// CI keeps the official URLs. Locally, default to npmmirror and reuse
// node_modules/electron/dist when it is already installed.
const { spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const inCI = !!(process.env.CI || process.env.GITHUB_ACTIONS);
if (!inCI) {
  if (!process.env.ELECTRON_MIRROR) {
    process.env.ELECTRON_MIRROR = "https://npmmirror.com/mirrors/electron/";
  }
  if (!process.env.ELECTRON_BUILDER_BINARIES_MIRROR) {
    process.env.ELECTRON_BUILDER_BINARIES_MIRROR =
      "https://npmmirror.com/mirrors/electron-builder-binaries/";
  }
}

const forwarded = process.argv.slice(2);
if (forwarded.length === 0) {
  console.error("usage: node scripts/with-electron-mirrors.cjs <command> [args...]");
  process.exit(2);
}

const dist = path.join(__dirname, "..", "node_modules", "electron", "dist");
const wantsWin =
  forwarded.includes("--win") ||
  (!forwarded.includes("--mac") && !forwarded.includes("--linux"));
if (
  process.platform === "win32" &&
  wantsWin &&
  fs.existsSync(path.join(dist, "electron.exe")) &&
  !forwarded.some((a) => a.startsWith("-c.electronDist") || a.startsWith("--config.electronDist"))
) {
  forwarded.push(`-c.electronDist=${dist}`);
}

if (process.platform === "win32" && wantsWin) {
  seedWinCodeSignCache();
  prefetchBuilderArtifact("nsis", "3.0.4.1");
  prefetchBuilderArtifact("nsis-resources", "3.4.1");
}

const command = forwarded[0];
const args = forwarded.slice(1);
const resolved =
  command === "electron-builder"
    ? require.resolve("electron-builder/cli.js")
    : command;

function seedWinCodeSignCache() {
  // winCodeSign-2.6.0.7z ships macOS libcrypto/libssl symlinks. A normal
  // Windows account cannot create them, so electron-builder retries 4 times
  // and dies. Seed the cache without the darwin half (same as docs/desktop.md).
  const cache = path.join(
    process.env.LOCALAPPDATA || "",
    "electron-builder",
    "Cache",
    "winCodeSign",
  );
  const dest = path.join(cache, "winCodeSign-2.6.0");
  const marker = path.join(dest, "windows-10");
  if (fs.existsSync(marker)) {
    return;
  }
  if (!fs.existsSync(cache)) {
    return;
  }
  const archive = fs
    .readdirSync(cache)
    .filter((name) => name.endsWith(".7z"))
    .map((name) => path.join(cache, name))
    .sort((a, b) => fs.statSync(b).mtimeMs - fs.statSync(a).mtimeMs)[0];
  if (!archive) {
    return;
  }
  const za = path.join(__dirname, "..", "node_modules", "7zip-bin", "win", "x64", "7za.exe");
  if (!fs.existsSync(za)) {
    return;
  }
  fs.rmSync(dest, { recursive: true, force: true });
  const extracted = spawnSync(
    za,
    ["x", "-snld", "-y", archive, `-o${dest}`, "-x!darwin"],
    { stdio: "inherit" },
  );
  if (extracted.status !== 0 || !fs.existsSync(marker)) {
    console.warn("with-electron-mirrors: could not seed winCodeSign cache (need Developer Mode, or extract without darwin — see docs/desktop.md)");
    return;
  }
  for (const name of fs.readdirSync(cache)) {
    if (/^\d+$/.test(name)) {
      fs.rmSync(path.join(cache, name), { recursive: true, force: true });
    }
  }
  console.log(`with-electron-mirrors: seeded ${dest} (darwin excluded)`);
}

function prefetchBuilderArtifact(name, version) {
  const cacheRoot = path.join(process.env.LOCALAPPDATA || "", "electron-builder", "Cache", name);
  const dest = path.join(cacheRoot, `${name}-${version}`);
  if (fs.existsSync(dest) && fs.readdirSync(dest).length > 0) {
    return;
  }
  const mirror = (
    process.env.ELECTRON_BUILDER_BINARIES_MIRROR ||
    "https://npmmirror.com/mirrors/electron-builder-binaries/"
  ).replace(/\/?$/, "/");
  const url = `${mirror}${name}-${version}/${name}-${version}.7z`;
  fs.mkdirSync(cacheRoot, { recursive: true });
  const archive = path.join(cacheRoot, `${name}-${version}.7z`);
  const curl = spawnSync(
    "curl.exe",
    ["-L", "--retry", "5", "--retry-all-errors", "--connect-timeout", "20", "--max-time", "180", "-o", archive, url],
    { stdio: "inherit" },
  );
  if (curl.status !== 0 || !fs.existsSync(archive) || fs.statSync(archive).size < 10000) {
    console.warn(`with-electron-mirrors: could not prefetch ${url}`);
    return;
  }
  const za = path.join(__dirname, "..", "node_modules", "7zip-bin", "win", "x64", "7za.exe");
  if (!fs.existsSync(za)) {
    return;
  }
  fs.rmSync(dest, { recursive: true, force: true });
  const extracted = spawnSync(za, ["x", "-snld", "-y", archive, `-o${dest}`], { stdio: "inherit" });
  if (extracted.status !== 0) {
    console.warn(`with-electron-mirrors: could not extract ${archive}`);
  }
}

const result = spawnSync(process.execPath, [resolved, ...args], {
  stdio: "inherit",
  env: process.env,
});
process.exit(result.status == null ? 1 : result.status);
