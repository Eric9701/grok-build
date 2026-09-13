(function (root) {
  const HIDDEN_SLASH = { "always-approve": true };
  const SOURCE_RANK = { initialize: 1, list: 2, acu: 3 };

  function cmdMeta(cmd) {
    return (cmd && (cmd._meta || cmd.meta)) || {};
  }

  function isClientOnly(cmd) {
    return !!cmdMeta(cmd).clientOnly;
  }

  function isSkill(cmd) {
    if (isClientOnly(cmd)) return false;
    const m = cmdMeta(cmd);
    return typeof m.path === "string" && !!m.path && typeof m.scope === "string" && !!m.scope;
  }

  function isWorkflow(cmd) {
    const m = cmdMeta(cmd);
    return !!(m.workflowSource || m.workflowPath);
  }

  function kindOf(cmd) {
    if (isClientOnly(cmd)) return "page";
    if (isSkill(cmd)) return "skill";
    if (isWorkflow(cmd)) return "flow";
    return "builtin";
  }

  function hintOf(cmd) {
    const input = cmd && cmd.input;
    if (!input) return "";
    return input.hint || (input.unstructured && input.unstructured.hint) || "";
  }

  function sanitizeCommands(raw) {
    const out = [];
    const seen = {};
    for (let i = 0; i < (raw || []).length; i++) {
      const cmd = raw[i];
      const name = cmd && String(cmd.name || "").trim();
      if (!name || HIDDEN_SLASH[name] || seen[name]) continue;
      seen[name] = true;
      out.push(cmd);
    }
    return out;
  }

  function getSlashQuery(text, caret) {
    const before = String(text || "").slice(0, caret);
    const m = before.match(/\/(\S*)$/);
    if (!m) return null;
    const slashIndex = before.length - m[0].length;
    if (slashIndex > 0 && !/\s/.test(before.charAt(slashIndex - 1))) return null;
    return { query: m[1], atStart: slashIndex === 0, slashIndex: slashIndex };
  }

  function filterCommands(commands, query) {
    const list = commands || [];
    const q = String(query || "").toLowerCase();
    if (!q) return list.slice();
    const prefix = [];
    const mid = [];
    const desc = [];
    for (let i = 0; i < list.length; i++) {
      const c = list[i];
      const name = String(c.name || "").toLowerCase();
      const plugin = String(cmdMeta(c).pluginName || "").toLowerCase();
      const text = String(c.description || "").toLowerCase();
      if (name.indexOf(q) === 0 || plugin.indexOf(q) === 0) prefix.push(c);
      else if (name.indexOf(q) >= 0 || plugin.indexOf(q) >= 0) mid.push(c);
      else if (text.indexOf(q) >= 0) desc.push(c);
    }
    return prefix.concat(mid, desc);
  }

  function applySlashPick(text, caret, name) {
    const hit = getSlashQuery(text, caret);
    if (!hit) return { text: "/" + name + " ", caret: name.length + 2 };
    const before = String(text).slice(0, hit.slashIndex) + "/" + name + " ";
    const after = String(text).slice(caret);
    return { text: before + after, caret: before.length };
  }

  function isSkillsBrowse(text) {
    return /^\/skills?\s*$/i.test(String(text || "").trim());
  }

  function shouldReplaceCatalog(prevRank, prevLen, source, nextLen) {
    const rank = SOURCE_RANK[source] || 0;
    if (rank < (prevRank || 0) && prevLen > 0) return false;
    if (source === "list" && rank === prevRank && nextLen < prevLen) return false;
    return true;
  }

  root.AtlasSlash = {
    HIDDEN_SLASH: HIDDEN_SLASH,
    SOURCE_RANK: SOURCE_RANK,
    cmdMeta: cmdMeta,
    isClientOnly: isClientOnly,
    isSkill: isSkill,
    isWorkflow: isWorkflow,
    kindOf: kindOf,
    hintOf: hintOf,
    sanitizeCommands: sanitizeCommands,
    getSlashQuery: getSlashQuery,
    filterCommands: filterCommands,
    applySlashPick: applySlashPick,
    isSkillsBrowse: isSkillsBrowse,
    shouldReplaceCatalog: shouldReplaceCatalog
  };
})(typeof window !== "undefined" ? window : globalThis);
