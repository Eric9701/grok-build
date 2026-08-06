#!/usr/bin/env node
// Node 18 shim: recent undici (pulled by @vscode/vsce) expects global File (Node 20+).
if (typeof globalThis.File === "undefined") {
  globalThis.File = class File {
    constructor(bits = [], name = "", options = {}) {
      this.bits = bits;
      this.name = name;
      this.options = options;
    }
  };
}
require("@vscode/vsce/vsce");
