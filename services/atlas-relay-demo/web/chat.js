const S = window.AtlasSlash;
    const MD = window.AtlasMD;
    const CLIENT_SKILLS = {
      name: "skills",
      description: "浏览本会话已加载的技能与斜杠（本页面板，不发给 Agent）",
      _meta: { clientOnly: true }
    };

    const logEl = document.getElementById("log");
    const cwdEl = document.getElementById("cwd");
    const modelEl = document.getElementById("modelId");
    const sessionIdEl = document.getElementById("sessionId");
    const applyModelBtn = document.getElementById("applyModelBtn");
    const newSessionBtn = document.getElementById("newSessionBtn");
    const loadSessionBtn = document.getElementById("loadSessionBtn");
    const histBtn = document.getElementById("histBtn");
    const histPanel = document.getElementById("histPanel");
    const histList = document.getElementById("histList");
    const histCount = document.getElementById("histCount");
    const histClose = document.getElementById("histClose");
    const inputEl = document.getElementById("input");
    const sendBtn = document.getElementById("sendBtn");
    const cancelBtn = document.getElementById("cancelBtn");
    const cmdBtn = document.getElementById("cmdBtn");
    const agentDot = document.getElementById("agentDot");
    const statusEl = document.getElementById("status");
    const agentSel = document.getElementById("agentSel");
    const jobIdEl = document.getElementById("jobId");
    const reqUrlEl = document.getElementById("reqUrl");
    const designUrlEl = document.getElementById("designUrl");
    const acceptUrlEl = document.getElementById("acceptUrl");
    const sampleBtn = document.getElementById("sampleBtn");
    const dispatchBtn = document.getElementById("dispatchBtn");
    const slashMenu = document.getElementById("slashMenu");
    const cmdPanel = document.getElementById("cmdPanel");
    const cmdList = document.getElementById("cmdList");
    const cmdFilter = document.getElementById("cmdFilter");
    const cmdCount = document.getElementById("cmdCount");
    const cmdClose = document.getElementById("cmdClose");
    const tabAll = document.getElementById("tabAll");
    const tabSkill = document.getElementById("tabSkill");
    const tabBuiltin = document.getElementById("tabBuiltin");

    const urlAgent = (new URLSearchParams(location.search).get("agent") || "").trim();
    let boundId = urlAgent;
    let agents = [];
    let ws;
    let rpcId = 1;
    let pending = new Map();
    const transcripts = {};
    const sessions = {};
    let streaming = null;
    let lastOnline = {};
    let ackedBound = "";
    let stolen = false;
    let slashHits = [];
    let slashIndex = 0;
    let slashQuery = "";
    let cmdTab = "all";
    let cmdOpen = false;

    function sessionOf(id) {
      if (!id) return { sessionId: null, handshake: "idle", busy: false, commands: [], commandSourceRank: 0 };
      if (!sessions[id]) {
        sessions[id] = { sessionId: null, handshake: "idle", busy: false, commands: [], commandSourceRank: 0 };
      }
      return sessions[id];
    }

    function resetSession(sess) {
      sess.handshake = "idle";
      sess.initialized = false;
      sess.sessionId = null;
      sess.busy = false;
      sess.commands = [];
      sess.commandSourceRank = 0;
    }

    const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

    function modelKey(id) {
      return id ? "atlas-relay-model." + id : "atlas-relay-model";
    }

    function loadModel(id) {
      const specific = id ? localStorage.getItem(modelKey(id)) : null;
      if (specific != null) return specific;
      return localStorage.getItem("atlas-relay-model") || "";
    }

    function saveModel(id, value) {
      if (id) localStorage.setItem(modelKey(id), value);
      localStorage.setItem("atlas-relay-model", value);
    }

    function cwdKey(id) {
      return id ? "atlas-relay-cwd." + id : "atlas-relay-cwd";
    }

    function loadCwd(id) {
      const specific = id ? localStorage.getItem(cwdKey(id)) : null;
      if (specific != null) return specific;
      return localStorage.getItem("atlas-relay-cwd") || "";
    }

    function saveCwd(id, value) {
      if (id) localStorage.setItem(cwdKey(id), value);
      localStorage.setItem("atlas-relay-cwd", value);
    }

    cwdEl.value = loadCwd(boundId);
    modelEl.value = loadModel(boundId);

    const setupFold = document.getElementById("setupFold");
    if (localStorage.getItem("atlas-relay-setup-open") === "0") setupFold.open = false;
    setupFold.addEventListener("toggle", () => {
      localStorage.setItem("atlas-relay-setup-open", setupFold.open ? "1" : "0");
    });

    function snapshotLog() {
      return [...logEl.children].map((el) => ({
        className: el.className,
        text: el.dataset.raw != null ? el.dataset.raw : el.textContent
      }));
    }

    function restoreLog(items) {
      logEl.innerHTML = "";
      streaming = null;
      for (const item of items || []) {
        const d = document.createElement("div");
        d.className = item.className;
        paintMsg(d, item.text || "");
        logEl.appendChild(d);
      }
      logEl.scrollTop = logEl.scrollHeight;
    }

    function saveTranscript() {
      if (!boundId) return;
      transcripts[boundId] = snapshotLog();
    }

    function shouldRenderMd(el) {
      if (!MD || !el) return false;
      if (el.classList.contains("think") || el.classList.contains("sys") ||
          el.classList.contains("err") || el.classList.contains("tool")) {
        return false;
      }
      return el.classList.contains("agent") || el.classList.contains("user");
    }

    function paintMsg(el, text) {
      el.dataset.raw = text;
      if (shouldRenderMd(el)) {
        el.classList.add("md");
        el.innerHTML = MD.render(text);
      } else {
        el.classList.remove("md");
        el.textContent = text;
      }
    }

    function appendMsg(el, chunk) {
      paintMsg(el, (el.dataset.raw || "") + chunk);
    }

    function add(kind, text, extraClass) {
      const d = document.createElement("div");
      d.className = "msg " + kind + (extraClass ? " " + extraClass : "");
      paintMsg(d, text);
      logEl.appendChild(d);
      logEl.scrollTop = logEl.scrollHeight;
      if (boundId) transcripts[boundId] = snapshotLog();
      return d;
    }

    function boundOnline() {
      if (!boundId) return false;
      return agents.some((a) => a.id === boundId && a.online);
    }

    const cmdMeta = S.cmdMeta;
    const isClientOnly = S.isClientOnly;
    const isSkill = S.isSkill;
    const kindOf = S.kindOf;
    const hintOf = S.hintOf;

    function kindLabel(kind) {
      return { page: "本页", skill: "技能", flow: "工作流", builtin: "内置" }[kind] || "";
    }

    function applyCommands(id, raw, source) {
      if (!id) return;
      const sess = sessionOf(id);
      const next = S.sanitizeCommands(raw);
      if (!S.shouldReplaceCatalog(sess.commandSourceRank || 0, (sess.commands || []).length, source, next.length)) {
        return;
      }
      sess.commands = next;
      sess.commandSourceRank = Math.max(sess.commandSourceRank || 0, S.SOURCE_RANK[source] || 0);
      refreshSlashUi();
      setStatus();
    }

    function advertisedCommands() {
      return sessionOf(boundId).commands || [];
    }

    function menuCommands() {
      return [CLIENT_SKILLS].concat(advertisedCommands());
    }

    const getSlashQuery = S.getSlashQuery;
    const filterCommands = S.filterCommands;
    const applySlashPick = S.applySlashPick;

    function commandCountText(sess) {
      const cmds = sess.commands || [];
      if (!cmds.length) return "";
      const skills = cmds.filter(isSkill).length;
      return " · " + skills + " 技能 / " + cmds.length + " 命令";
    }

    function setStatus() {
      const sess = sessionOf(boundId);
      const online = boundOnline();
      agentDot.classList.toggle("on", online && sess.handshake === "ready");
      const sessTxt = sess.sessionId ? " · session " + sess.sessionId.slice(0, 8) : "";
      if (!agents.some((a) => a.online) && !boundId) {
        statusEl.textContent = "没有在线 Agent — 先启动 atlas agent headless";
      } else if (stolen) {
        statusEl.textContent = boundId + " 已被其他页面占用，请重新选择";
      } else if (!boundId) {
        statusEl.textContent = "请选择要对话的 Agent";
      } else if (!online) {
        statusEl.textContent = boundId + " 离线 — 保持绑定，等待重连";
      } else if (sess.handshake !== "ready") {
        statusEl.textContent = boundId + " 已连接，正在建立 ACP 会话…";
      } else {
        statusEl.textContent = "就绪 · " + boundId + sessTxt + commandCountText(sess);
      }
      sendBtn.disabled = stolen || !online || sess.handshake !== "ready" || sess.busy;
      dispatchBtn.disabled = sendBtn.disabled;
      cancelBtn.disabled = !sess.busy;
      cmdBtn.disabled = stolen || !boundId;
    }

    function renderCmdRow(cmd) {
      const kind = kindOf(cmd);
      const wrap = document.createElement("div");
      const name = document.createElement("div");
      name.className = "name";
      name.appendChild(document.createTextNode("/" + cmd.name));
      const badge = document.createElement("span");
      badge.className = "badge " + kind;
      badge.textContent = kindLabel(kind);
      name.appendChild(badge);
      const plugin = cmdMeta(cmd).pluginName;
      if (plugin) {
        const extra = document.createElement("span");
        extra.className = "badge";
        extra.textContent = plugin;
        name.appendChild(extra);
      }
      const desc = document.createElement("div");
      desc.className = "desc";
      const hint = hintOf(cmd);
      desc.textContent = [cmd.description || "", hint ? "参数 " + hint : ""].filter(Boolean).join(" · ");
      wrap.appendChild(name);
      if (desc.textContent) wrap.appendChild(desc);
      return wrap;
    }

    function hideSlashMenu() {
      slashMenu.classList.remove("open");
      slashHits = [];
      slashIndex = 0;
      slashQuery = "";
      slashMenu.innerHTML = "";
    }

    function renderSlashMenu() {
      const caret = inputEl.selectionStart || 0;
      const hit = getSlashQuery(inputEl.value, caret);
      if (!hit) {
        hideSlashMenu();
        return;
      }
      if (hit.query !== slashQuery) {
        slashQuery = hit.query;
        slashIndex = 0;
      }
      slashHits = filterCommands(menuCommands(), hit.query).slice(0, 16);
      if (!slashHits.length) {
        hideSlashMenu();
        return;
      }
      if (slashIndex >= slashHits.length) slashIndex = 0;
      slashMenu.innerHTML = "";
      slashHits.forEach((cmd, i) => {
        const row = renderCmdRow(cmd);
        row.className = "slash-item" + (i === slashIndex ? " on" : "");
        row.addEventListener("mousedown", (e) => {
          e.preventDefault();
          pickSlash(cmd);
        });
        slashMenu.appendChild(row);
      });
      slashMenu.classList.add("open");
    }

    function refreshSlashUi() {
      if (slashMenu.classList.contains("open")) renderSlashMenu();
      if (cmdOpen) renderCmdPanel();
    }

    function pickSlash(cmd) {
      if (isClientOnly(cmd) || cmd.name === "skills") {
        hideSlashMenu();
        openCmdPanel("skill");
        return;
      }
      const next = applySlashPick(inputEl.value, inputEl.selectionStart || 0, cmd.name);
      inputEl.value = next.text;
      hideSlashMenu();
      inputEl.focus();
      inputEl.setSelectionRange(next.caret, next.caret);
    }

    function renderCmdPanel() {
      const q = cmdFilter.value.trim();
      let items = advertisedCommands();
      if (cmdTab === "skill") items = items.filter(isSkill);
      else if (cmdTab === "builtin") items = items.filter((c) => kindOf(c) === "builtin");
      items = filterCommands(items, q);
      cmdCount.textContent = items.length + " / " + advertisedCommands().length;
      tabAll.classList.toggle("active", cmdTab === "all");
      tabSkill.classList.toggle("active", cmdTab === "skill");
      tabBuiltin.classList.toggle("active", cmdTab === "builtin");
      cmdList.innerHTML = "";
      if (!advertisedCommands().length) {
        const empty = document.createElement("p");
        empty.className = "cmd-empty";
        empty.textContent = sessionOf(boundId).handshake === "ready"
          ? "会话已建立，但还没有收到斜杠目录。等 Agent 推 available_commands_update，或检查插件是否 enabled、cwd 是否为业务仓根。"
          : "先选 Agent 并等到圆点变绿。目录来自本机 CLI，Relay 不扫磁盘。";
        cmdList.appendChild(empty);
        return;
      }
      if (!items.length) {
        const empty = document.createElement("p");
        empty.className = "cmd-empty";
        empty.textContent = "没有匹配的命令";
        cmdList.appendChild(empty);
        return;
      }
      for (const cmd of items) {
        const row = renderCmdRow(cmd);
        row.className = "cmd-item";
        row.addEventListener("click", () => {
          inputEl.value = "/" + cmd.name + (hintOf(cmd) ? " " : "");
          closeCmdPanel();
          inputEl.focus();
          const pos = inputEl.value.length;
          inputEl.setSelectionRange(pos, pos);
        });
        cmdList.appendChild(row);
      }
    }

    function openCmdPanel(tab) {
      cmdTab = tab || cmdTab || "all";
      cmdOpen = true;
      cmdPanel.classList.add("open");
      cmdPanel.setAttribute("aria-hidden", "false");
      renderCmdPanel();
      cmdFilter.focus();
    }

    function closeCmdPanel() {
      cmdOpen = false;
      cmdPanel.classList.remove("open");
      cmdPanel.setAttribute("aria-hidden", "true");
    }

    function toggleCmdPanel() {
      if (cmdOpen) closeCmdPanel();
      else openCmdPanel(cmdTab);
    }

    function rebuildSelect() {
      const keep = boundId;
      agentSel.innerHTML = "";
      const ph = document.createElement("option");
      ph.value = "";
      ph.textContent = agents.length ? "选择 Agent…" : "等待 Agent…";
      agentSel.appendChild(ph);
      for (const a of agents) {
        const opt = document.createElement("option");
        opt.value = a.id;
        const extra = [a.userId, a.version].filter(Boolean).join(" · ");
        opt.textContent = a.id + (a.online ? "" : "（离线）") + (extra ? "  " + extra : "");
        agentSel.appendChild(opt);
      }
      agentSel.value = keep && [...agentSel.options].some((o) => o.value === keep) ? keep : "";
    }

    function rememberAgentInUrl(id) {
      const u = new URL(location.href);
      if (id) u.searchParams.set("agent", id);
      else u.searchParams.delete("agent");
      history.replaceState(null, "", u);
    }

    function sendRpc(method, params) {
      const id = rpcId++;
      const msg = { jsonrpc: "2.0", id, method, params };
      ws.send(JSON.stringify(msg));
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
      });
    }

    function requestBind(id, fromUser) {
      id = (id || "").trim();
      if (!id) return;
      stolen = false;
      if (id !== boundId) {
        saveTranscript();
        boundId = id;
        cwdEl.value = loadCwd(id);
        modelEl.value = loadModel(id);
        restoreLog(transcripts[id] || []);
        streaming = null;
        hideSlashMenu();
        if (cmdOpen) renderCmdPanel();
      } else {
        boundId = id;
      }
      rememberAgentInUrl(id);
      rebuildSelect();
      setStatus();
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          jsonrpc: "2.0",
          method: "atlas.relay/bind",
          params: { agentId: id }
        }));
      }
      const sess = sessionOf(id);
      if (boundOnline() && sess.initialized) refreshModelList(id);
      if (boundOnline() && sess.handshake === "idle" && !typedSessionId()) startSession();
      if (fromUser && !boundOnline()) add("sys", "已选择 " + id + "（离线，等待连接）");
    }

    function connect() {
      const url = (location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws/client";
      ws = new WebSocket(url);
      ws.onopen = () => {
        add("sys", "已连上 relay");
        if (boundId) requestBind(boundId);
      };
      ws.onclose = () => {
        for (const [, p] of pending) p.reject(new Error("relay 断开"));
        pending.clear();
        add("sys", "与 relay 断开，2s 后重连");
        setTimeout(connect, 2000);
      };
      ws.onerror = () => {};
      ws.onmessage = (ev) => {
        let msg;
        try { msg = JSON.parse(ev.data); } catch { return; }
        handle(msg);
      };
    }

    async function pullCommandList(id, cwd) {
      const sess = sessionOf(id);
      if (!sess.sessionId) return;
      try {
        const listed = await sendRpc("x.ai/commands/list", {
          sessionId: sess.sessionId,
          cwd
        });
        if (boundId !== id) return;
        applyCommands(id, listed.commands || [], "list");
      } catch {
        // ACU 会再推一次；ext 方法不可用时保持 initialize 的 builtin 列表
      }
    }

    function typedSessionId() {
      return (sessionIdEl.value || "").trim();
    }

    function currentModel() {
      return (modelEl.value || "").trim();
    }

    function sessionMeta(extra) {
      const meta = Object.assign({ yoloMode: true }, extra || {});
      const model = currentModel();
      if (model) meta.modelId = model;
      return meta;
    }

    function currentCwd(id) {
      const cwd = (cwdEl.value || "").trim() || ".";
      saveCwd(id, cwdEl.value || "");
      saveModel(id, modelEl.value || "");
      return cwd;
    }

    async function ensureInit(id) {
      const sess = sessionOf(id);
      if (sess.initialized) return true;
      const init = await sendRpc("initialize", {
        protocolVersion: 1,
        clientCapabilities: {
          fs: { readTextFile: false, writeTextFile: false },
          terminal: false
        },
        _meta: {
          clientIdentifier: "atlas-relay-demo",
          clientType: "atlas-relay-demo",
          startupHints: { nonInteractive: true }
        }
      });
      if (boundId !== id) return false;
      const initMeta = (init && (init._meta || init.meta)) || {};
      applyCommands(id, initMeta.availableCommands || [], "initialize");
      sess.initialized = true;
      fillModelSelect(initMeta.modelState || {});
      refreshModelList(id);
      return true;
    }

    function unwrapModelState(payload) {
      if (!payload || typeof payload !== "object") return {};
      if (payload.error) {
        const err = payload.error;
        throw new Error(typeof err === "string" ? err : (err.message || JSON.stringify(err)));
      }
      const body = payload.result && typeof payload.result === "object" ? payload.result : payload;
      if (body.modelState && typeof body.modelState === "object") return body.modelState;
      return body;
    }

    function fillModelSelect(state) {
      const models = (state && (state.availableModels || state.available_models)) || [];
      const current = (state && (state.currentModelId || state.current_model_id)) || "";
      const keep = modelEl.value || loadModel(boundId);
      modelEl.textContent = "";
      const def = document.createElement("option");
      def.value = "";
      def.textContent = "默认（Agent 启动模型）";
      modelEl.appendChild(def);
      const seen = new Set();
      for (const m of models) {
        const id = (m && (m.modelId || m.model_id)) || "";
        if (!id || seen.has(id)) continue;
        seen.add(id);
        const opt = document.createElement("option");
        opt.value = id;
        const name = (m.name || id).trim();
        opt.textContent = id === current ? name + "（当前）" : name;
        if (m.description) opt.title = m.description;
        modelEl.appendChild(opt);
      }
      if (keep && !seen.has(keep)) {
        const extra = document.createElement("option");
        extra.value = keep;
        extra.textContent = keep;
        modelEl.appendChild(extra);
        seen.add(keep);
      }
      modelEl.value = keep && seen.has(keep) ? keep : "";
    }

    async function refreshModelList(id) {
      if (!id || boundId !== id || !boundOnline()) return;
      try {
        const listed = await sendRpc("x.ai/models/list", {});
        if (boundId !== id) return;
        fillModelSelect(unwrapModelState(listed));
      } catch (e) {
        if (boundId === id && modelEl.options.length <= 1) add("err", "模型列表失败 · " + (e.message || e));
      }
    }

    function adoptSession(id, sess, sessionId, cwd, how) {
      sess.sessionId = sessionId;
      sess.handshake = "ready";
      sess.busy = false;
      add("sys", how + " · " + id + " · " + sessionId + " · cwd = " + cwd);
      pullCommandList(id, cwd);
    }

    async function startSession() {
      const id = boundId;
      const sess = sessionOf(id);
      if (!id || !boundOnline() || sess.handshake === "init") return;
      if (sess.handshake === "ready") return;
      await openNewSession(false);
    }

    async function openNewSession(explicit) {
      const id = boundId;
      const sess = sessionOf(id);
      if (!id || !boundOnline() || sess.handshake === "init" || sess.busy) return;
      const requested = explicit ? typedSessionId() : "";
      if (requested && !UUID_RE.test(requested)) {
        add("err", "会话 id 必须是 UUID");
        return;
      }
      const prev = sess.handshake;
      sess.handshake = "init";
      setStatus();
      try {
        if (!(await ensureInit(id))) return;
        if (boundId !== id) return;
        const cwd = currentCwd(id);
        const meta = sessionMeta(requested ? { sessionId: requested } : null);
        const res = await sendRpc("session/new", {
          cwd,
          mcpServers: [],
          _meta: meta
        });
        if (boundId !== id) return;
        const sessionId = res.sessionId || (res.session && res.session.id);
        if (!sessionId) throw new Error("session/new 没有返回 sessionId: " + JSON.stringify(res));
        adoptSession(id, sess, sessionId, cwd, requested ? "已按指定 id 新建" : "会话已建立");
      } catch (e) {
        if (boundId === id && sess.handshake === "init") sess.handshake = prev === "ready" ? "ready" : "idle";
        add("err", String(e.message || e));
      }
      setStatus();
    }

    async function loadNamedSession() {
      const id = boundId;
      const sess = sessionOf(id);
      if (!id || !boundOnline() || sess.handshake === "init" || sess.busy) return;
      const requested = typedSessionId();
      if (!requested) {
        add("err", "载入需要填写会话 id");
        return;
      }
      if (!UUID_RE.test(requested)) {
        add("err", "会话 id 必须是 UUID");
        return;
      }
      const prev = sess.handshake;
      sess.handshake = "init";
      setStatus();
      try {
        if (!(await ensureInit(id))) return;
        if (boundId !== id) return;
        const cwd = currentCwd(id);
        const res = await sendRpc("session/load", {
          sessionId: requested,
          cwd,
          mcpServers: []
        });
        if (boundId !== id) return;
        const sessionId = (res && (res.sessionId || (res.session && res.session.id))) || requested;
        adoptSession(id, sess, sessionId, cwd, "已载入会话");
        const model = currentModel();
        if (model) {
          await sendRpc("session/set_model", { sessionId, modelId: model });
          if (boundId === id) add("sys", "已切换模型 · " + model);
        }
      } catch (e) {
        if (boundId === id && sess.handshake === "init") sess.handshake = prev === "ready" ? "ready" : "idle";
        add("err", String(e.message || e));
      }
      setStatus();
    }

    async function applyModel() {
      const id = boundId;
      const sess = sessionOf(id);
      const model = currentModel();
      saveModel(id, modelEl.value || "");
      if (!model) {
        add("err", "先从列表选择模型");
        return;
      }
      if (!sess.sessionId || sess.handshake !== "ready") {
        add("sys", "模型将在下次新建或载入时使用 · " + model);
        return;
      }
      try {
        await sendRpc("session/set_model", { sessionId: sess.sessionId, modelId: model });
        add("sys", "已切换模型 · " + model);
      } catch (e) {
        add("err", String(e.message || e));
      }
    }

    function sessionRows(payload) {
      if (payload && payload.error) {
        const err = payload.error;
        throw new Error(typeof err === "string" ? err : (err.message || JSON.stringify(err)));
      }
      const body = payload && payload.result && typeof payload.result === "object"
        ? payload.result
        : payload;
      const raw = body && (body.sessions || body.rows || []);
      return Array.isArray(raw) ? raw : [];
    }

    function sessionTitle(row) {
      return (row.title || row.summary || row.firstPrompt || row.sessionId || "").trim();
    }

    function shortWhen(value) {
      if (!value) return "";
      const d = new Date(value);
      if (Number.isNaN(d.getTime())) return value;
      const p = (n) => String(n).padStart(2, "0");
      return d.getFullYear() + "-" + p(d.getMonth() + 1) + "-" + p(d.getDate())
        + " " + p(d.getHours()) + ":" + p(d.getMinutes());
    }

    function closeHistPanel() {
      histPanel.classList.remove("open");
      histPanel.setAttribute("aria-hidden", "true");
    }

    function renderHistory(rows) {
      histList.textContent = "";
      histCount.textContent = rows.length ? rows.length + " 条" : "";
      if (!rows.length) {
        const empty = document.createElement("p");
        empty.className = "cmd-empty";
        empty.textContent = "没有会话。确认 cwd 是该 Agent 机器上的目录。";
        histList.appendChild(empty);
        return;
      }
      for (const row of rows) {
        const id = row.sessionId || row.session_id;
        if (!id) continue;
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "cmd-item";
        const name = document.createElement("span");
        name.className = "name";
        name.textContent = sessionTitle(row) || id;
        const desc = document.createElement("span");
        desc.className = "desc";
        const when = shortWhen(row.updatedAt || row.lastActiveAt || "");
        const model = row.modelId ? " · " + row.modelId : "";
        desc.textContent = id + (when ? " · " + when : "") + model + (row.cwd ? " · " + row.cwd : "");
        btn.appendChild(name);
        btn.appendChild(desc);
        btn.addEventListener("click", () => {
          sessionIdEl.value = id;
          if (row.cwd) cwdEl.value = row.cwd;
          closeHistPanel();
          loadNamedSession();
        });
        histList.appendChild(btn);
      }
    }

    async function showHistory() {
      const id = boundId;
      if (!id || !boundOnline()) {
        add("err", "先绑定一个在线 Agent");
        return;
      }
      const sess = sessionOf(id);
      if (sess.handshake === "init" || sess.busy) {
        add("sys", "会话正在建立，稍后再看历史");
        return;
      }
      if (cmdOpen) closeCmdPanel();
      histPanel.classList.add("open");
      histPanel.setAttribute("aria-hidden", "false");
      histList.textContent = "";
      const hint = document.createElement("p");
      hint.className = "cmd-empty";
      hint.textContent = "正在读取…";
      histList.appendChild(hint);
      const prev = sess.handshake;
      if (!sess.initialized) {
        sess.handshake = "init";
        setStatus();
      }
      try {
        if (!(await ensureInit(id))) return;
        if (boundId !== id) return;
        const cwd = (cwdEl.value || "").trim();
        const listed = await sendRpc("x.ai/session/list", {
          cwd: cwd || undefined,
          headless: "include",
          limit: 40,
          allowRelax: true
        });
        if (boundId !== id) return;
        renderHistory(sessionRows(listed));
      } catch (e) {
        histList.textContent = "";
        const err = document.createElement("p");
        err.className = "cmd-empty";
        err.textContent = String(e.message || e);
        histList.appendChild(err);
      }
      if (boundId === id && sess.handshake === "init" && !sess.sessionId) {
        sess.handshake = prev === "ready" ? "ready" : "idle";
      }
      setStatus();
    }

    function handleStatus(params) {
      agents = params.agents || [];
      const serverBound = (params.boundAgentId || "").trim();
      if (serverBound) {
        stolen = false;
        if (serverBound !== boundId) {
          saveTranscript();
          boundId = serverBound;
          cwdEl.value = loadCwd(boundId);
          modelEl.value = loadModel(boundId);
          restoreLog(transcripts[boundId] || []);
          rememberAgentInUrl(boundId);
          hideSlashMenu();
        }
        ackedBound = serverBound;
      } else if (ackedBound && ackedBound === boundId) {
        add("sys", boundId + " 已被其他页面占用，请重新选择");
        ackedBound = "";
        stolen = true;
      }
      rebuildSelect();

      const onlineIds = agents.filter((a) => a.online).map((a) => a.id);
      if (!boundId && !stolen && onlineIds.length === 1) {
        requestBind(onlineIds[0]);
        return;
      }

      const online = boundOnline();
      const was = lastOnline[boundId];
      lastOnline[boundId] = online;
      const sess = sessionOf(boundId);
      if (boundId && was && !online) {
        resetSession(sess);
        hideSlashMenu();
        add("sys", boundId + " 已离线，绑定保留");
      }
      if (boundId && online && sess.handshake === "idle" && !typedSessionId()) startSession();
      setStatus();
      if (cmdOpen) renderCmdPanel();
    }

    function handle(msg) {
      if (msg.method === "atlas.relay/status") {
        handleStatus(msg.params || {});
        return;
      }
      if (msg.method === "session/request_permission") {
        autoAllow(msg);
        return;
      }
      if (msg.method === "x.ai/models/update") {
        const state = unwrapModelState(msg.params || {});
        const models = state.availableModels || state.available_models || [];
        if (models.length) fillModelSelect(state);
        return;
      }
      if (msg.method === "session/update" || msg.method === "x.ai/session_notification") {
        onUpdate(msg.params && (msg.params.update || msg.params));
        return;
      }
      if (msg.id != null && pending.has(msg.id)) {
        const p = pending.get(msg.id);
        pending.delete(msg.id);
        if (msg.error) p.reject(new Error(msg.error.message || JSON.stringify(msg.error)));
        else p.resolve(msg.result || {});
        return;
      }
    }

    function autoAllow(msg) {
      const opts = (msg.params && msg.params.options) || [];
      const pick = opts.find((o) => o.kind === "allow_once" || o.kind === "allowOnce") || opts[0];
      const result = pick
        ? { outcome: { outcome: "selected", optionId: pick.optionId } }
        : { outcome: { outcome: "cancelled" } };
      ws.send(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result }));
      const title = (msg.params && msg.params.toolCall && (msg.params.toolCall.title || msg.params.toolCall.kind)) || "工具";
      add("tool", "已自动允许：" + title);
    }

    function onUpdate(u) {
      if (!u) return;
      const kind = u.sessionUpdate;
      if (kind === "available_commands_update") {
        applyCommands(boundId, u.availableCommands || u.available_commands || [], "acu");
        return;
      }
      if (kind === "agent_message_chunk") {
        const t = (u.content && u.content.text) || "";
        if (!streaming) streaming = add("agent", "");
        appendMsg(streaming, t);
        if (boundId) transcripts[boundId] = snapshotLog();
        logEl.scrollTop = logEl.scrollHeight;
      } else if (kind === "agent_thought_chunk") {
        const t = (u.content && u.content.text) || "";
        if (!streaming) streaming = add("agent", "", "think");
        appendMsg(streaming, t);
        if (boundId) transcripts[boundId] = snapshotLog();
      } else if (kind === "tool_call") {
        streaming = null;
        const title = u.title || u.kind || "tool";
        add("tool", "▶ " + title);
      } else if (kind === "tool_call_update") {
        const st = u.status || "";
        if (st && st !== "pending" && st !== "in_progress") {
          add("tool", "■ " + (u.title || u.toolCallId || "tool") + " · " + st);
        }
      }
    }

    function lastAgentText() {
      const nodes = [...logEl.querySelectorAll(".msg.agent")];
      if (!nodes.length) return "";
      const el = nodes[nodes.length - 1];
      return el.dataset.raw || el.textContent;
    }

    function extractReport(text) {
      const i = text.indexOf("{");
      const j = text.lastIndexOf("}");
      if (i < 0 || j <= i) return null;
      try {
        const obj = JSON.parse(text.slice(i, j + 1));
        return obj && obj.jobId ? obj : null;
      } catch {
        return null;
      }
    }

    const isSkillsBrowse = S.isSkillsBrowse;

    async function sendPromptText(text) {
      const sess = sessionOf(boundId);
      if (!text || sess.handshake !== "ready" || sess.busy) return false;
      add("user", text);
      streaming = null;
      sess.busy = true;
      hideSlashMenu();
      setStatus();
      try {
        await sendRpc("session/prompt", {
          sessionId: sess.sessionId,
          prompt: [{ type: "text", text }]
        });
        return true;
      } catch (e) {
        add("err", String(e.message || e));
        return false;
      } finally {
        streaming = null;
        sess.busy = false;
        setStatus();
      }
    }

    async function sendPrompt() {
      const text = inputEl.value.trim();
      if (!text) return;
      if (isSkillsBrowse(text)) {
        inputEl.value = "";
        hideSlashMenu();
        openCmdPanel("skill");
        return;
      }
      inputEl.value = "";
      hideSlashMenu();
      await sendPromptText(text);
      inputEl.focus();
    }

    function fillSampleUrls() {
      const base = location.origin + "/sample-docs/";
      if (!jobIdEl.value.trim()) jobIdEl.value = "demo-" + Date.now().toString(36);
      reqUrlEl.value = base + "requirements.md";
      designUrlEl.value = base + "design.md";
      acceptUrlEl.value = base + "acceptance.md";
    }

    async function runDispatch() {
      const sess = sessionOf(boundId);
      if (sess.handshake !== "ready" || sess.busy || stolen) return;
      const body = {
        jobId: jobIdEl.value.trim() || "demo-" + Date.now().toString(36),
        requirementsUrl: reqUrlEl.value.trim(),
        designUrl: designUrlEl.value.trim(),
        acceptanceUrl: acceptUrlEl.value.trim()
      };
      jobIdEl.value = body.jobId;
      let prompts;
      try {
        const res = await fetch("/dispatch/prompts", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        });
        const raw = await res.text();
        if (!res.ok) throw new Error(raw || res.statusText);
        prompts = JSON.parse(raw);
      } catch (e) {
        add("err", "派工文案失败：" + (e.message || e));
        return;
      }
      add("sys", "派工 " + prompts.jobId + " · 三轮：落盘 → Role 4 → 回执");
      if (!await sendPromptText(prompts.drop)) return;
      if (!await sendPromptText(prompts.execute)) return;
      if (!await sendPromptText(prompts.report)) return;
      const report = extractReport(lastAgentText());
      if (report) {
        add("sys", "回执 " + (report.status || "?") + " · " + (report.summary || prompts.reportPath));
      } else {
        add("sys", "未解析到 JSON 回执，请看 Agent 原文或 " + prompts.reportPath);
      }
    }

    function cancelTurn() {
      const sess = sessionOf(boundId);
      if (!sess.sessionId || !ws || ws.readyState !== WebSocket.OPEN) return;
      ws.send(JSON.stringify({
        jsonrpc: "2.0",
        method: "session/cancel",
        params: { sessionId: sess.sessionId }
      }));
    }

    sendBtn.onclick = sendPrompt;
    cancelBtn.onclick = cancelTurn;
    cmdBtn.onclick = toggleCmdPanel;
    cmdClose.onclick = closeCmdPanel;
    sampleBtn.onclick = fillSampleUrls;
    dispatchBtn.onclick = runDispatch;
    tabAll.onclick = () => { cmdTab = "all"; renderCmdPanel(); };
    tabSkill.onclick = () => { cmdTab = "skill"; renderCmdPanel(); };
    tabBuiltin.onclick = () => { cmdTab = "builtin"; renderCmdPanel(); };
    cmdFilter.addEventListener("input", renderCmdPanel);
    agentSel.addEventListener("change", () => {
      if (agentSel.value) requestBind(agentSel.value, true);
    });
    inputEl.addEventListener("input", renderSlashMenu);
    inputEl.addEventListener("click", renderSlashMenu);
    inputEl.addEventListener("keydown", (e) => {
      const open = slashMenu.classList.contains("open");
      if (open && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
        e.preventDefault();
        if (!slashHits.length) return;
        slashIndex = (slashIndex + (e.key === "ArrowDown" ? 1 : slashHits.length - 1)) % slashHits.length;
        renderSlashMenu();
        return;
      }
      if (open && (e.key === "Tab" || (e.key === "Enter" && !e.shiftKey))) {
        e.preventDefault();
        if (slashHits[slashIndex]) pickSlash(slashHits[slashIndex]);
        return;
      }
      if (e.key === "Escape") {
        if (open) {
          e.preventDefault();
          hideSlashMenu();
        } else if (histPanel.classList.contains("open")) {
          e.preventDefault();
          closeHistPanel();
        } else if (cmdOpen) {
          e.preventDefault();
          closeCmdPanel();
        }
        return;
      }
      if (e.key === "Enter" && !e.shiftKey) {
        if (window.matchMedia("(pointer: coarse)").matches) return;
        e.preventDefault();
        sendPrompt();
      }
    });
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && document.activeElement !== inputEl) {
        if (histPanel.classList.contains("open")) closeHistPanel();
        else if (cmdOpen) closeCmdPanel();
      }
    });
    cwdEl.addEventListener("change", () => saveCwd(boundId, cwdEl.value));
    modelEl.addEventListener("change", () => saveModel(boundId, modelEl.value));
    newSessionBtn.addEventListener("click", () => openNewSession(true));
    loadSessionBtn.addEventListener("click", () => loadNamedSession());
    applyModelBtn.addEventListener("click", () => applyModel());
    histBtn.addEventListener("click", () => showHistory());
    histClose.addEventListener("click", () => closeHistPanel());

    setStatus();
    connect();
