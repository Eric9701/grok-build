const assert = require("assert");
require("./md.js");
const R = globalThis.AtlasMD.render;

assert.ok(R("hello **bold**").indexOf("<strong>bold</strong>") >= 0);
assert.ok(R("use `code`").indexOf("<code>code</code>") >= 0);
assert.ok(R("# Title").indexOf("<h1>Title</h1>") >= 0);
assert.ok(R("## Sub").indexOf("<h2>Sub</h2>") >= 0);

const list = R("- a\n- b");
assert.ok(list.indexOf("<ul>") >= 0);
assert.ok(list.indexOf("<li>a</li>") >= 0);

const ol = R("1. one\n2. two");
assert.ok(ol.indexOf("<ol>") >= 0);

const fence = R("```js\nconst x = 1 < 2\n```");
assert.ok(fence.indexOf("<pre><code class=\"lang-js\">") >= 0);
assert.ok(fence.indexOf("&lt;") >= 0);
assert.ok(fence.indexOf("<script") < 0);

assert.ok(R("<script>alert(1)</script>").indexOf("<script") < 0);
assert.ok(R("[x](javascript:alert(1))").indexOf("javascript:") < 0);
assert.ok(R("[ok](https://example.com)").indexOf('href="https://example.com"') >= 0);
assert.ok(R("see https://example.com/a").indexOf('href="https://example.com/a"') >= 0);

const table = R("| a | b |\n| --- | --- |\n| 1 | 2 |");
assert.ok(table.indexOf("<table>") >= 0);
assert.ok(table.indexOf("<th>a</th>") >= 0);
assert.ok(table.indexOf("<td>1</td>") >= 0);

assert.ok(R("> quote").indexOf("<blockquote>") >= 0);
assert.ok(R("---").indexOf("<hr") >= 0);
assert.ok(R("- [x] done").indexOf("checked") >= 0);

const open = R("```\nstill streaming");
assert.ok(open.indexOf("<pre><code>") >= 0);
assert.ok(open.indexOf("still streaming") >= 0);

const br = R("line1\nline2");
assert.ok(br.indexOf("<br") >= 0);

console.log("ok");
