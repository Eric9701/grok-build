/**
 * User-appended `[model.<id>]` tables in the global Atlas `config.toml`.
 *
 * The CLI merges these over remote ListModels / builtin catalog. Project
 * `.atlas/config.toml` does not read `[model.*]` — only the global file.
 *
 * Section-aware line scan (no TOML dependency). Managed tables
 * (`managed = true`) are listed separately and never overwritten.
 */

export const LOCAL_MODEL_ID_PATTERN = /^[A-Za-z0-9._-]+$/;

export interface LocalModelDraft {
  id: string;
  model?: string;
  name?: string;
  description?: string;
  baseUrl?: string;
  /** Omit to keep an existing key on upsert; `""` clears it. */
  apiKey?: string;
  envKey?: string;
  contextWindow?: number;
}

/** Safe to send to a webview — never includes `api_key`. */
export interface LocalModelView {
  id: string;
  model?: string;
  name?: string;
  description?: string;
  baseUrl?: string;
  envKey?: string;
  hasApiKey: boolean;
  contextWindow?: number;
}

export class LocalModelError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "LocalModelError";
  }
}

interface TomlChunk {
  /** Original header line, or empty for the file preamble. */
  headerLine: string;
  body: string;
}

export function isValidLocalModelId(id: string): boolean {
  return LOCAL_MODEL_ID_PATTERN.test(id);
}

export function parseModelTableId(headerInner: string): string | undefined {
  const trimmed = headerInner.trim();
  if (!trimmed.toLowerCase().startsWith("model.")) return undefined;
  const rest = trimmed.slice("model.".length).trim();
  if (!rest) return undefined;
  if (rest.startsWith('"')) {
    const parsed = unquoteToml(rest);
    return parsed || undefined;
  }
  if (!LOCAL_MODEL_ID_PATTERN.test(rest)) return undefined;
  return rest;
}

export function listUserLocalModels(toml: string): LocalModelView[] {
  return parseModelTables(toml)
    .filter((entry) => !entry.managed)
    .map((entry) => toView(entry.id, entry.fields));
}

export function upsertUserLocalModel(toml: string, draft: LocalModelDraft): string {
  const id = draft.id.trim();
  if (!isValidLocalModelId(id)) {
    throw new LocalModelError(
      `Invalid model id "${id}". Use letters, digits, dots, underscores, or hyphens.`,
    );
  }
  const chunks = splitTomlChunks(toml);
  const index = chunks.findIndex((chunk) => modelIdOf(chunk) === id);
  if (index >= 0) {
    const existing = fieldsOf(chunks[index].body);
    if (isTruthyToml(existing.managed)) {
      throw new LocalModelError(`Cannot overwrite managed model "${id}".`);
    }
    const merged = mergeFields(existing, draft);
    chunks[index] = { headerLine: `[model.${id}]`, body: formatFields(merged) };
    return joinTomlChunks(chunks);
  }
  const next = [...chunks];
  const body = formatFields(mergeFields({}, draft));
  if (next.length === 1 && !next[0].headerLine && !next[0].body.trim()) {
    return `[model.${id}]\n${body}`;
  }
  next.push({ headerLine: `[model.${id}]`, body });
  return joinTomlChunks(next);
}

export function removeUserLocalModel(toml: string, id: string): string {
  const target = id.trim();
  if (!isValidLocalModelId(target)) {
    throw new LocalModelError(`Invalid model id "${target}".`);
  }
  const chunks = splitTomlChunks(toml);
  const index = chunks.findIndex((chunk) => modelIdOf(chunk) === target);
  if (index < 0) throw new LocalModelError(`No local model named "${target}".`);
  if (isTruthyToml(fieldsOf(chunks[index].body).managed)) {
    throw new LocalModelError(`Cannot remove managed model "${target}".`);
  }
  chunks.splice(index, 1);
  return joinTomlChunks(chunks);
}

export function toLocalModelView(draft: LocalModelDraft, hasApiKey: boolean): LocalModelView {
  return {
    id: draft.id,
    model: emptyToUndef(draft.model),
    name: emptyToUndef(draft.name),
    description: emptyToUndef(draft.description),
    baseUrl: emptyToUndef(draft.baseUrl),
    envKey: emptyToUndef(draft.envKey),
    hasApiKey,
    contextWindow: draft.contextWindow,
  };
}

function parseModelTables(toml: string): Array<{ id: string; managed: boolean; fields: Record<string, string> }> {
  const out: Array<{ id: string; managed: boolean; fields: Record<string, string> }> = [];
  for (const chunk of splitTomlChunks(toml)) {
    const id = modelIdOf(chunk);
    if (!id) continue;
    const fields = fieldsOf(chunk.body);
    out.push({ id, managed: isTruthyToml(fields.managed), fields });
  }
  return out;
}

function toView(id: string, fields: Record<string, string>): LocalModelView {
  const windowRaw = fields.context_window?.trim();
  const windowNum = windowRaw ? Number(windowRaw) : NaN;
  return {
    id,
    model: emptyToUndef(fields.model),
    name: emptyToUndef(fields.name),
    description: emptyToUndef(fields.description),
    baseUrl: emptyToUndef(fields.base_url),
    envKey: emptyToUndef(fields.env_key),
    hasApiKey: !!emptyToUndef(fields.api_key),
    contextWindow: Number.isFinite(windowNum) && windowNum > 0 ? windowNum : undefined,
  };
}

function mergeFields(existing: Record<string, string>, draft: LocalModelDraft): Record<string, string> {
  const next = { ...existing };
  setOrDelete(next, "model", draft.model);
  setOrDelete(next, "name", draft.name);
  setOrDelete(next, "description", draft.description);
  setOrDelete(next, "base_url", draft.baseUrl);
  setOrDelete(next, "env_key", draft.envKey);
  if (draft.apiKey !== undefined) setOrDelete(next, "api_key", draft.apiKey);
  if (draft.contextWindow != null && Number.isFinite(draft.contextWindow) && draft.contextWindow > 0) {
    next.context_window = String(Math.round(draft.contextWindow));
  } else if (draft.contextWindow === 0) {
    delete next.context_window;
  }
  delete next.managed;
  return next;
}

function setOrDelete(fields: Record<string, string>, key: string, value: string | undefined): void {
  if (value === undefined) return;
  const trimmed = value.trim();
  if (!trimmed) delete fields[key];
  else fields[key] = trimmed;
}

function formatFields(fields: Record<string, string>): string {
  const order = ["model", "name", "description", "base_url", "api_key", "env_key", "context_window"];
  const lines: string[] = [];
  const seen = new Set<string>();
  for (const key of order) {
    if (fields[key] == null || fields[key] === "") continue;
    seen.add(key);
    lines.push(formatTomlAssignment(key, fields[key], key === "context_window"));
  }
  for (const [key, value] of Object.entries(fields)) {
    if (seen.has(key) || value == null || value === "") continue;
    lines.push(formatTomlAssignment(key, value, false));
  }
  return lines.length ? `${lines.join("\n")}\n` : "";
}

function formatTomlAssignment(key: string, value: string, numeric: boolean): string {
  if (numeric && /^-?\d+(\.\d+)?$/.test(value)) return `${key} = ${value}`;
  if (value === "true" || value === "false") return `${key} = ${value}`;
  return `${key} = ${quoteToml(value)}`;
}

function splitTomlChunks(toml: string): TomlChunk[] {
  const lines = toml.split(/\r?\n/);
  const chunks: TomlChunk[] = [];
  let headerLine = "";
  let bodyLines: string[] = [];
  const flush = () => {
    chunks.push({ headerLine, body: bodyLines.join("\n") });
    bodyLines = [];
  };
  for (const line of lines) {
    if (isTableHeader(line)) {
      flush();
      headerLine = line;
    } else {
      bodyLines.push(line);
    }
  }
  flush();
  return chunks;
}

function joinTomlChunks(chunks: TomlChunk[]): string {
  const parts: string[] = [];
  for (const chunk of chunks) {
    if (chunk.headerLine) {
      if (parts.length && !parts[parts.length - 1].endsWith("\n\n")) {
        const last = parts[parts.length - 1];
        if (last && !last.endsWith("\n")) parts[parts.length - 1] = last + "\n";
        if (parts[parts.length - 1] && !parts[parts.length - 1].endsWith("\n\n")) {
          parts.push("");
        }
      }
      parts.push(chunk.headerLine);
    }
    if (chunk.body) parts.push(chunk.body);
  }
  let out = parts.join("\n");
  out = out.replace(/\n{3,}/g, "\n\n");
  if (out && !out.endsWith("\n")) out += "\n";
  return out;
}

function isTableHeader(line: string): boolean {
  const trimmed = line.replace(/#.*$/, "").trim();
  return /^\[[^\]]+\]$/.test(trimmed) || /^\[\[[^\]]+\]\]$/.test(trimmed);
}

function headerInner(headerLine: string): string | undefined {
  const trimmed = headerLine.replace(/#.*$/, "").trim();
  const m = trimmed.match(/^\[\[?\s*([^\]]+?)\s*\]\]?$/);
  return m?.[1]?.trim();
}

function modelIdOf(chunk: TomlChunk): string | undefined {
  if (!chunk.headerLine) return undefined;
  const inner = headerInner(chunk.headerLine);
  if (!inner) return undefined;
  return parseModelTableId(inner);
}

function fieldsOf(body: string): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const raw of body.split(/\r?\n/)) {
    const line = raw.replace(/#.*$/, "").trim();
    if (!line || line.startsWith("[")) continue;
    const eq = line.indexOf("=");
    if (eq < 1) continue;
    const key = line.slice(0, eq).trim();
    if (!key) continue;
    fields[key] = unquoteToml(line.slice(eq + 1).trim());
  }
  return fields;
}

function isTruthyToml(value: string | undefined): boolean {
  return (value || "").trim().toLowerCase() === "true";
}

function quoteToml(value: string): string {
  return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

function unquoteToml(raw: string): string {
  const value = raw.trim();
  if (
    (value.startsWith('"') && value.endsWith('"') && value.length >= 2) ||
    (value.startsWith("'") && value.endsWith("'") && value.length >= 2)
  ) {
    return value.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, "\\");
  }
  return value;
}

function emptyToUndef(value: string | undefined): string | undefined {
  const trimmed = (value || "").trim();
  return trimmed || undefined;
}
