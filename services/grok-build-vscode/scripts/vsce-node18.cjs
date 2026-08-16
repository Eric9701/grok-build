#!/usr/bin/env node
// Node 18 shim for @vscode/vsce 3.9 (engines: node >= 20).
//
// 1) undici expects global File (Node 20+).
// 2) vsce runs `npm list --production --parseable` and treats any non-zero
//    exit as fatal. On Windows that often fails with ELSPROBLEMS:
//    electron-builder leftovers (extraneous) and optional @openai/codex-*
//    platform stubs (invalid). stdout is still a usable path list; .vscodeignore
//    then drops everything except ws / jpeg-js / the Codex adapter.
const cp = require("child_process");

if (typeof globalThis.File === "undefined") {
  globalThis.File = class File {
    constructor(bits = [], name = "", options = {}) {
      this.bits = bits;
      this.name = name;
      this.options = options;
    }
  };
}

const origExec = cp.exec;
cp.exec = function patchedExec(command, options, callback) {
  if (typeof options === "function") {
    callback = options;
    options = {};
  }
  const isNpmList =
    typeof command === "string" && /npm list --production --parseable/.test(command);
  if (!isNpmList || typeof callback !== "function") {
    return origExec.apply(this, arguments);
  }
  return origExec.call(this, command, options, (err, stdout, stderr) => {
    if (
      err &&
      stdout &&
      String(stdout).trim() &&
      /ELSPROBLEMS|extraneous:|invalid:/.test(String(stderr || err.message || ""))
    ) {
      return callback(null, stdout, stderr);
    }
    return callback(err, stdout, stderr);
  });
};

require("@vscode/vsce/vsce");
