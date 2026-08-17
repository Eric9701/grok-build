# Desktop auto-update

This is the current-state contract for Atlas Desktop updates. Phase 1
(notice-only) shipped first. Phase 2 adds a real `electron-updater` download
on macOS and Windows under the same rail affordance. Rules from phase 1 carry
unless a later section overrides them.

The feed is **not** GitHub Releases. A vsix-only tag becomes GitHub's
"latest" and would stall a GitHub-provider updater. The relay serves the
newest release that actually carries desktop installers.

## Phase 1 — notice (still the fallback)

The main process checks on first paint (4 s delay) and every 12 hours
(`DESKTOP_UPDATE_CHECK_INTERVAL_MS`). Failure is silence: offline, 404,
rate-limit, or a malformed body never raise a dialog.

The check lists GitHub Releases (`DESKTOP_RELEASES_API_URL`) and
`noticeIfUpdateAvailable` picks the newest non-draft release that carries a
desktop installer (`isDesktopInstallerAsset`: anchored
`-mac-arm64.dmg` / `-mac-x64.dmg` / `-mac-arm64.zip` / `-mac-x64.zip` /
`-win-x64.exe`; `.blockmap` and `.vsix` do not count). Pre-releases count.
The running version is compared with `isNewerVersion` (numeric semver, not
string order).

When newer, the host posts a host-local `updateAvailable` frame
`{ version, url }`. The URL is always `desktopUpdatePageUrl(current)` —
`http://10.218.220.237:22255/atlas/cli?from=<version>` — never the GitHub
release page. The rail button says **Update available**; the panel opens
that page (`openUpdateRelease`, host-local inbound).

No persistent state beyond an in-memory pending notice re-posted after a
document reload. VS Code never sends the frame.

## Phase 2 — in-app download

Packaged Windows and macOS builds download in the background through
`electron-updater` and install on quit or when the user clicks the same
rail button, now labelled **Restart to update**.

### Client feed (generic provider)

Do **not** use the GitHub provider. Point `electron-updater` at the same
host Atlas CLI already uses (`GROK_CLI_BASE_URL`, default
`http://10.218.220.237:22255/atlas/cli`):

| Platform | Generic `url` (directory, trailing slash) | File the updater GETs |
|---|---|---|
| Windows (`win32`) | `http://10.218.220.237:22255/atlas/cli/` | `…/atlas/cli/latest.yml` |
| macOS (`darwin`) | `http://10.218.220.237:22255/atlas/cli/` | `…/atlas/cli/latest-mac.yml` |

Win and mac share one directory: atlas-server `/cli/*` only serves a
single path segment, so `latest.yml` / `latest-mac.yml` sit next to CLI
binaries in `releases/`. `GROK_CLI_BASE_URL` overrides the origin at
runtime (`setFeedURL`); the baked `app-update.yml` uses the same default.

Those paths are `desktopUpdateFeedBase` / `desktopUpdateFeedConfig` in
`src/desktop/app-update.ts`. Linux has no feed; the client stays on the
phase-1 notice.

`allowPrerelease` is irrelevant: whichever `latest.yml` you publish is
what the client installs. The client does not filter channels.

### Relay contract

Serve `GET /atlas/cli/latest.yml` and `GET /atlas/cli/latest-mac.yml` as
`application/x-yaml` (or `text/yaml` / `text/plain`). Each file is the
`electron-builder` channel yml. Keep `sha512` / `size`. Prefer
**relative** `files[].url` names (the installer basename) so
electron-updater resolves them against `/atlas/cli/` — the same static
tree that already hosts `grok-{ver}-{os}-{arch}` CLI binaries.

Do **not** invent blockmap entries. electron-updater derives
`{fileUrl}.blockmap` from the installer URL; drop those files next to
the installer in `releases/` if you want delta updates.

Windows `latest.yml` must list the NSIS installer
(`Grok-Build-Desktop-<version>-win-x64.exe`). macOS `latest-mac.yml` must
list **both** zip archives (`…-mac-arm64.zip` and `…-mac-x64.zip`).
Squirrel.Mac updates from the zip, not the dmg.

404, empty body, or unparseable YAML: the client logs and falls back to
the phase-1 notice. Do not serve HTML error pages with status 200.

### Feed service

Put the yml files and installer artifacts in atlas-server `releases/`
(same directory as CLI channel pointers). A missing yml is a 404 and
the client falls back to the GitHub notice. Deleting `latest.yml` is the
operator kill-switch for a bad Windows update.

Any check or download failure (not just 404) falls back to the phase-1
notice. With Windows signature verification off, the atlas-server
`/atlas/cli` tree is the trust boundary for what gets installed on quit.

### What this repo emits

`electron-builder.yml` has a generic `publish` block **only so the yml
files are generated**. Every `dist*` script still passes `--publish never`.
GitHub upload stays in `.github/workflows/desktop-release.yml`, which
attaches `dist-desktop/latest.yml` and `dist-desktop/latest-mac.yml`
alongside the installers.

macOS builds both arches in **one** `electron-builder --mac` invocation
(`dist:mac`). The workflow then asserts `latest-mac.yml` contains both
zip names. Do not split mac arches across jobs: the second write would
drop the first arch (electron-builder issue #5592).

### Client behaviour

1. Packaged win32/darwin: configure the generic feed, `autoDownload = true`,
   `autoInstallOnAppQuit = true`. Check after first paint and every 12 h.
2. Update found: download in the background. **No dialog. No button yet.**
   The agent may be mid-turn.
3. Download finished: post host-local `updateReady { version }`. The rail
   button the notice taught users now says **Restart to update**. Click
   posts host-local `restartToUpdate`; the host calls `quitAndInstall`.
   Ordinary quit also installs.
4. Check or download fails (offline, 404, malformed yml, hash mismatch,
   network drop): log only, then run the phase-1 GitHub notice. The rail
   shows **Update available** and opens the download page.
5. Unpackaged `npm run desktop`, Linux, or a missing feed: phase-1 notice
   only.
6. No new persistent state beyond what electron-updater writes itself
   (its cache under userData).

`updateAvailable` and `updateReady` are host-local outbound. A remote
never sees the button. `openUpdateRelease` and `restartToUpdate` are
host-local inbound.

### Windows signatures

The app is unsigned until an Authenticode cert lands. **Do not set
`publisherName`.** `win.verifyUpdateCodeSignature: false` in
`electron-builder.yml` is the operative switch: it keeps `publisherName`
out of the baked `app-update.yml`. A runtime assignment is a no-op
(NsisUpdater's setter ignores falsy) and is not used. Unsigned → signed
updates in place are fine later — but verification does NOT turn on by
itself: when the cert lands, REMOVE `win.verifyUpdateCodeSignature: false`
so electron-builder bakes `publisherName` and the updater verifies.

Per-user NSIS (the default) needs no elevation. Users who installed under
Program Files via `allowToChangeInstallationDirectory` get a UAC "unknown
publisher" prompt per update — owner-approved.

macOS builds are signed and notarised (Developer ID, Team L6TFKRX6QQ);
Squirrel.Mac's signature requirement is satisfied. Zip targets already
ship for both arches.

### Manual test (unpackaged only)

A packaged build never reads `dev-app-update.yml`. `setFeedURL` of the
production feed would override that file even with `forceDevUpdateConfig`
set, so the documented local-feed path is unpackaged-only: when
`GROK_DESKTOP_UPDATE_DEV=1` and the app is not packaged, the client sets
`forceDevUpdateConfig` and does **not** call `setFeedURL`. electron-updater
then reads `dev-app-update.yml` from the app path.

electron-updater is not unit-testable. To exercise a real download:

1. Serve a local generic feed. Example layout:

   ```
   http://127.0.0.1:8765/update/win/latest.yml
   http://127.0.0.1:8765/update/mac/latest-mac.yml
   ```

   `files[].url` may be absolute `http://127.0.0.1:8765/…` installer URLs.
   Keep a valid `sha512` of the file you serve.

2. Put `dev-app-update.yml` in **`out/desktop/`** (NOT the repo root):

   ```yaml
   provider: generic
   url: http://127.0.0.1:8765/update/win/
   ```

   Use `/update/mac/` on a Mac. electron-updater reads the file from
   `app.getAppPath()`, and this repo launches `electron out/desktop/main.js`,
   so `getAppPath()` is `out/desktop`. `tsc` does not copy the file — create
   it there after compiling (it is inside gitignored `out/`; the root-level
   `dev-app-update.yml` gitignore entry is belt for a misplaced copy).

3. Launch unpackaged (`npm run desktop`) with `GROK_DESKTOP_UPDATE_DEV=1`.
   Set the feed's `version` above **Electron's own version** (unpackaged
   `app.getVersion()` is Electron's, e.g. `33.2.1` — not `package.json`'s;
   use e.g. `99.0.0`). Confirm: log line on check, silent download, rail
   **Restart to update**, click restarts into the new build. Then kill the
   server and confirm a later check stays silent. Do NOT expect the GitHub
   notice unpackaged: it compares against the same Electron `currentVersion`,
   and no `3.x` release is newer than `33.2.1` — the notice fallback is only
   observable in a packaged build.

Do not enable `forceDevUpdateConfig` in a packaged build.
