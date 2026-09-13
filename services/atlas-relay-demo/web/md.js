(function (root) {
  function escapeHtml(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function escapeAttr(s) {
    return escapeHtml(s).replace(/'/g, "&#39;");
  }

  function safeHref(href) {
    const t = String(href || "").trim();
    if (/^https?:\/\//i.test(t) || /^mailto:/i.test(t)) return t;
    return "";
  }

  function fenceLang(info) {
    const m = String(info || "").trim().match(/^([A-Za-z0-9_+-]+)/);
    return m ? m[1].toLowerCase() : "";
  }

  function extractFences(src) {
    const fences = [];
    const lines = src.split("\n");
    const out = [];
    let i = 0;
    while (i < lines.length) {
      const open = lines[i].match(/^(`{3,}|~{3,})(.*)$/);
      if (!open) {
        out.push(lines[i]);
        i += 1;
        continue;
      }
      const mark = open[1];
      const lang = fenceLang(open[2]);
      const body = [];
      i += 1;
      let closed = false;
      while (i < lines.length) {
        if (lines[i].slice(0, mark.length) === mark && !lines[i].slice(mark.length).trim()) {
          closed = true;
          i += 1;
          break;
        }
        body.push(lines[i]);
        i += 1;
      }
      const token = "\u0001MD" + fences.length + "\u0001";
      fences.push({ lang: lang, code: body.join("\n"), closed: closed });
      out.push(token);
    }
    return { text: out.join("\n"), fences: fences };
  }

  function restoreFences(html, fences) {
    let out = html;
    for (let i = 0; i < fences.length; i++) {
      const f = fences[i];
      const lang = f.lang ? ' class="lang-' + escapeAttr(f.lang) + '"' : "";
      const block =
        "<pre><code" + lang + ">" + escapeHtml(f.code) + "</code></pre>";
      out = out.split("\u0001MD" + i + "\u0001").join(block);
    }
    return out;
  }

  function renderInline(s) {
    const bits = [];
    function keep(html) {
      const token = "\u0001IN" + bits.length + "\u0001";
      bits.push(html);
      return token;
    }
    s = String(s || "");
    s = s.replace(/`([^`\n]+)`/g, function (_, code) {
      return keep("<code>" + escapeHtml(code) + "</code>");
    });
    s = s.replace(/!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, function (_, alt, href) {
      const url = safeHref(href);
      if (!url) return alt || href;
      return keep(
        '<img src="' + escapeAttr(url) + '" alt="' + escapeAttr(alt) + '" />'
      );
    });
    s = s.replace(/\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, function (_, label, href) {
      const url = safeHref(href);
      if (!url) return label;
      return keep(
        '<a href="' +
          escapeAttr(url) +
          '" target="_blank" rel="noopener noreferrer">' +
          escapeHtml(label) +
          "</a>"
      );
    });
    s = s.replace(/(^|[\s(])((https?:\/\/)[^\s<)]+)/g, function (_, lead, url) {
      if (!safeHref(url)) return lead + url;
      return (
        lead +
        keep(
          '<a href="' +
            escapeAttr(url) +
            '" target="_blank" rel="noopener noreferrer">' +
            escapeHtml(url) +
            "</a>"
        )
      );
    });
    s = escapeHtml(s);
    s = s.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
    s = s.replace(/__(.+?)__/g, "<strong>$1</strong>");
    s = s.replace(/~~(.+?)~~/g, "<del>$1</del>");
    s = s.replace(/(^|[^\w*])\*(?!\s)([^*\n]+?)\*(?!\*)/g, "$1<em>$2</em>");
    for (let i = 0; i < bits.length; i++) {
      s = s.split("\u0001IN" + i + "\u0001").join(bits[i]);
    }
    return s;
  }

  function isHr(line) {
    return /^(?:-{3,}|\*{3,}|_{3,})$/.test(line.trim());
  }

  function heading(line) {
    const m = line.match(/^(#{1,6})\s+(.+)$/);
    if (!m) return null;
    return { level: m[1].length, text: m[2].replace(/\s+#+\s*$/, "") };
  }

  function listItem(line) {
    const m = line.match(/^(\s*)([-*+]|\d+\.)\s+(?:\[([ xX])\]\s+)?(.*)$/);
    if (!m) return null;
    const ordered = /\d+\./.test(m[2]);
    const task = m[3] != null;
    return {
      indent: m[1].length,
      ordered: ordered,
      task: task,
      checked: m[3] === "x" || m[3] === "X",
      text: m[4]
    };
  }

  function tableRow(line) {
    const t = line.trim();
    if (t.charAt(0) !== "|") return null;
    const cells = t.replace(/^\||\|$/g, "").split("|").map(function (c) {
      return c.trim();
    });
    return cells;
  }

  function isTableSep(cells) {
    if (!cells || !cells.length) return false;
    return cells.every(function (c) {
      return /^:?-{3,}:?$/.test(c);
    });
  }

  function flushPara(buf) {
    if (!buf.length) return "";
    return "<p>" + buf.map(renderInline).join("<br />") + "</p>";
  }

  function renderList(items) {
    if (!items.length) return "";
    const ordered = items[0].ordered;
    const tag = ordered ? "ol" : "ul";
    const body = items
      .map(function (it) {
        let inner = renderInline(it.text);
        if (it.task) {
          inner =
            '<input type="checkbox" disabled' +
            (it.checked ? " checked" : "") +
            " /> " +
            inner;
        }
        return "<li>" + inner + "</li>";
      })
      .join("");
    return "<" + tag + ">" + body + "</" + tag + ">";
  }

  function renderQuote(lines) {
    const inner = lines
      .map(function (l) {
        return l.replace(/^>\s?/, "");
      })
      .join("\n");
    return "<blockquote>" + renderBlocks(inner) + "</blockquote>";
  }

  function renderTable(header, rows) {
    const th = header
      .map(function (c) {
        return "<th>" + renderInline(c) + "</th>";
      })
      .join("");
    const tr = rows
      .map(function (r) {
        const td = header
          .map(function (_, i) {
            return "<td>" + renderInline(r[i] || "") + "</td>";
          })
          .join("");
        return "<tr>" + td + "</tr>";
      })
      .join("");
    return "<table><thead><tr>" + th + "</tr></thead><tbody>" + tr + "</tbody></table>";
  }

  function renderBlocks(src) {
    const lines = String(src || "").split("\n");
    const out = [];
    let i = 0;
    let para = [];

    function endPara() {
      if (para.length) {
        out.push(flushPara(para));
        para = [];
      }
    }

    while (i < lines.length) {
      const line = lines[i];
      const tokenOnly = /^\u0001MD\d+\u0001$/.test(line.trim());
      if (!line.trim()) {
        endPara();
        i += 1;
        continue;
      }
      if (tokenOnly) {
        endPara();
        out.push(line.trim());
        i += 1;
        continue;
      }
      const h = heading(line);
      if (h) {
        endPara();
        out.push("<h" + h.level + ">" + renderInline(h.text) + "</h" + h.level + ">");
        i += 1;
        continue;
      }
      if (isHr(line)) {
        endPara();
        out.push("<hr />");
        i += 1;
        continue;
      }
      if (/^>\s?/.test(line)) {
        endPara();
        const q = [];
        while (i < lines.length && (/^>\s?/.test(lines[i]) || (q.length && !lines[i].trim()))) {
          q.push(lines[i]);
          i += 1;
        }
        out.push(renderQuote(q));
        continue;
      }
      const li = listItem(line);
      if (li && li.indent < 4) {
        endPara();
        const items = [];
        const ordered = li.ordered;
        while (i < lines.length) {
          const next = listItem(lines[i]);
          if (!next || next.ordered !== ordered || next.indent >= 4) break;
          items.push(next);
          i += 1;
        }
        out.push(renderList(items));
        continue;
      }
      const cells = tableRow(line);
      const sep = i + 1 < lines.length ? tableRow(lines[i + 1]) : null;
      if (cells && isTableSep(sep)) {
        endPara();
        const rows = [];
        i += 2;
        while (i < lines.length) {
          const row = tableRow(lines[i]);
          if (!row) break;
          rows.push(row);
          i += 1;
        }
        out.push(renderTable(cells, rows));
        continue;
      }
      para.push(line);
      i += 1;
    }
    endPara();
    return out.join("");
  }

  function render(src) {
    const normalized = String(src || "").replace(/\r\n/g, "\n");
    if (!normalized) return "";
    const extracted = extractFences(normalized);
    const html = renderBlocks(extracted.text);
    return restoreFences(html, extracted.fences);
  }

  root.AtlasMD = { render: render, escapeHtml: escapeHtml, safeHref: safeHref };
})(typeof window !== "undefined" ? window : globalThis);
