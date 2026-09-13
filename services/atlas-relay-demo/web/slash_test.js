const assert = require("assert");
require("./slash.js");
const S = globalThis.AtlasSlash;

const commit = {
  name: "commit",
  description: "Create a git commit",
  _meta: { path: "/home/u/.atlas/skills/commit/SKILL.md", scope: "user" }
};
const role4 = {
  name: "atlas-sdd:4-software-engineer-agent",
  description: "Implement from design",
  meta: { path: "/plugins/atlas-sdd/skills/4/SKILL.md", scope: "plugin", pluginName: "atlas-sdd" }
};
const compact = { name: "compact", description: "Compress history", input: { hint: "what to keep" } };
const yolo = { name: "always-approve", description: "hidden" };
const flow = { name: "ship", description: "Ship it", _meta: { workflowPath: "workflows/ship.md" } };

assert.strictEqual(S.kindOf(commit), "skill");
assert.strictEqual(S.kindOf(role4), "skill");
assert.strictEqual(S.kindOf(compact), "builtin");
assert.strictEqual(S.kindOf(flow), "flow");
assert.strictEqual(S.kindOf({ name: "skills", _meta: { clientOnly: true } }), "page");
assert.strictEqual(S.hintOf(compact), "what to keep");

const cleaned = S.sanitizeCommands([yolo, compact, compact, { name: "" }, commit]);
assert.deepStrictEqual(cleaned.map((c) => c.name), ["compact", "commit"]);

assert.ok(S.getSlashQuery("/com", 4));
assert.strictEqual(S.getSlashQuery("/com", 4).query, "com");
assert.strictEqual(S.getSlashQuery("see foo/bar", 11), null);
assert.ok(S.getSlashQuery("run /commit now", 11));

const filtered = S.filterCommands([compact, commit, role4], "sdd");
assert.strictEqual(filtered[0].name, role4.name);

const picked = S.applySlashPick("/com", 4, "compact");
assert.strictEqual(picked.text, "/compact ");

assert.ok(S.isSkillsBrowse("/skills"));
assert.ok(S.isSkillsBrowse("/skill"));
assert.ok(!S.isSkillsBrowse("/skill commit"));

assert.ok(S.shouldReplaceCatalog(1, 2, "acu", 10));
assert.ok(!S.shouldReplaceCatalog(3, 10, "initialize", 3));
assert.ok(!S.shouldReplaceCatalog(2, 10, "list", 2));

console.log("ok");
