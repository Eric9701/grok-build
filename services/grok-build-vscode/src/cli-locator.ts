import { existsSync } from "node:fs";
import { execSync } from "node:child_process";
import { homedir } from "node:os";
import * as path from "node:path";

const IS_WIN = process.platform === "win32";

/** Preferred Atlas binary names, then legacy `grok` for older installs. */
function candidateNames(preferAtlas: boolean): string[] {
  const atlas = IS_WIN ? ["atlas.cmd", "atlas.exe", "atlas.bat", "atlas"] : ["atlas"];
  const grok = IS_WIN ? ["grok.cmd", "grok.exe", "grok.bat", "grok"] : ["grok"];
  return preferAtlas ? [...atlas, ...grok] : [...grok, ...atlas];
}

function effectiveHome(): string {
  // Respect env overrides first so tests + users can redirect the home lookup.
  const fromEnv = IS_WIN ? process.env.USERPROFILE : process.env.HOME;
  return fromEnv || homedir();
}

/** Home dir bins to search: `$GROK_HOME/bin`, then `~/.atlas/bin`, then `~/.grok/bin`. */
function homeBinDirs(): string[] {
  const dirs: string[] = [];
  const seen = new Set<string>();
  const push = (p: string) => {
    const n = path.normalize(p);
    if (seen.has(n)) return;
    seen.add(n);
    dirs.push(n);
  };
  if (process.env.GROK_HOME) {
    push(path.join(process.env.GROK_HOME, "bin"));
  }
  const home = effectiveHome();
  push(path.join(home, ".atlas", "bin"));
  push(path.join(home, ".grok", "bin"));
  return dirs;
}

function findInDir(dir: string, names: string[]): string | undefined {
  for (const name of names) {
    const candidate = path.join(dir, name);
    if (existsSync(candidate)) return candidate;
  }
  return undefined;
}

function findOnPath(cmdName: string): string | undefined {
  try {
    const cmd = IS_WIN ? `where ${cmdName}` : `command -v ${cmdName}`;
    const out = execSync(cmd, { encoding: "utf8" }).trim();
    const first = out.split(/\r?\n/)[0]?.trim();
    if (first && existsSync(first)) return first;
  } catch {
    // ignore — not on PATH
  }
  return undefined;
}

/**
 * Locate the Atlas CLI (preferred) or legacy `grok` binary.
 * Order: configured path → `$GROK_HOME/bin` / `~/.atlas/bin` / `~/.grok/bin`
 * (atlas first) → PATH `atlas` → PATH `grok`.
 */
export function locateGrokCli(configuredPath: string): string | undefined {
  return locateAtlasCli(configuredPath);
}

/** Alias for {@link locateGrokCli} — Atlas-first discovery. */
export function locateAtlasCli(configuredPath: string): string | undefined {
  if (configuredPath) {
    return existsSync(configuredPath) ? configuredPath : undefined;
  }
  const names = candidateNames(true);
  for (const dir of homeBinDirs()) {
    const hit = findInDir(dir, names);
    if (hit) return hit;
  }
  return findOnPath("atlas") || findOnPath("grok");
}

/** Whether this activation follows a previously-recorded extension version. */
export function extensionWasUpgraded(lastSeen: string | undefined, current: string): boolean {
  return !!lastSeen && !!current && lastSeen !== current;
}

/**
 * Oldest CLI whose ACP behavior this extension supports. Native
 * exit_plan_mode verdicts are part of this contract, so every platform must
 * meet this floor. It is also the Windows-verified reactive recovery target.
 */
export const GROK_REQUIRED_VERSION = "0.2.117";

// Reactive Windows stdio recovery lands on the same fully-supported baseline.
export const GROK_STDIO_DOWNGRADE_TARGET = GROK_REQUIRED_VERSION;

/**
 * Parse a CLI `--version` banner ("atlas 0.2.64 …" / "grok 0.2.64 …") into a
 * `[major, minor, patch]` tuple, or undefined when no `X.Y.Z` is present. Pure.
 */
export function parseGrokVersion(versionOutput: string): [number, number, number] | undefined {
  const m = /(\d+)\.(\d+)\.(\d+)/.exec(versionOutput ?? "");
  if (!m) return undefined;
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

/** Compare two `[major, minor, patch]` tuples: <0, 0, or >0. Pure. */
export function compareVersionTuple(a: [number, number, number], b: [number, number, number]): number {
  return a[0] - b[0] || a[1] - b[1] || a[2] - b[2];
}

/** Windows grok/atlas 0.2.61-0.2.70 can hang before ACP startup completes. */
export function isStdioBrokenGrokVersion(versionOutput: string, platform: NodeJS.Platform): boolean {
  if (platform !== "win32") return false;
  const version = parseGrokVersion(versionOutput);
  if (!version) return false;
  const [major, minor, patch] = version;
  return major === 0 && minor === 2 && patch >= 61 && patch <= 70;
}

/** Whether a parseable installed CLI is older than the behavior baseline. */
export function isGrokVersionBelowRequired(versionOutput: string): boolean {
  const installed = parseGrokVersion(versionOutput);
  const required = parseGrokVersion(GROK_REQUIRED_VERSION)!;
  return !!installed && compareVersionTuple(installed, required) < 0;
}

/**
 * Decision for "Update Atlas CLI" (manual and extension-upgrade updates),
 * given the installed version + platform.
 */
export interface GrokUpdatePolicy {
  /** May the update run at all? */
  allow: boolean;
  /** When allowed, pin to this exact version instead of `latest` (undefined ⇒ latest). */
  target?: string;
  /** When blocked, the reason to surface in the menu / log. */
  note?: string;
}

export function grokUpdatePolicy(_versionOutput: string, _platform: NodeJS.Platform): GrokUpdatePolicy {
  // Updates are no longer gated for #22 (fixed in 0.2.71). Always allow, to latest.
  return { allow: true };
}

/**
 * Should the host REACTIVELY downgrade the CLI after an *observed* `agent stdio`
 * init failure (handshake timeout / "exited code null")?
 *
 * Windows-only (the regression is). A build at/below the target is never
 * reactively replaced; that is the loop guard once recovery lands. Pure.
 */
export function shouldReactivelyDowngrade(versionOutput: string, platform: NodeJS.Platform): boolean {
  if (platform !== "win32") return false;
  const v = parseGrokVersion(versionOutput);
  if (!v) return false;
  const target = parseGrokVersion(GROK_STDIO_DOWNGRADE_TARGET)!;
  return compareVersionTuple(v, target) > 0;
}

/**
 * Does a failed `atlas update` / `grok update` error mean the binary was still locked?
 * Pure.
 */
export function isLockedBinaryError(message: string): boolean {
  return /locked executable|os error 5|access is denied/i.test(message);
}
