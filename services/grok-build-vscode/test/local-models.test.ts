import { describe, expect, it } from "vitest";
import {
  LocalModelError,
  isValidLocalModelId,
  listUserLocalModels,
  parseModelTableId,
  removeUserLocalModel,
  upsertUserLocalModel,
} from "../src/local-models";

const BASE = `# Atlas global configuration

[ui]
permission_mode = "ask"

[models]
default = "grok-build"
`;

describe("isValidLocalModelId", () => {
  it("accepts dotted and hyphenated picker ids", () => {
    expect(isValidLocalModelId("local-llama")).toBe(true);
    expect(isValidLocalModelId("gpt-4o")).toBe(true);
    expect(isValidLocalModelId("acme.internal.v1")).toBe(true);
  });

  it("rejects empty, spaces, and TOML-hostile characters", () => {
    expect(isValidLocalModelId("")).toBe(false);
    expect(isValidLocalModelId("my model")).toBe(false);
    expect(isValidLocalModelId("foo]")).toBe(false);
  });
});

describe("parseModelTableId", () => {
  it("reads [model.id] headers and ignores [models]", () => {
    expect(parseModelTableId("model.local-llama")).toBe("local-llama");
    expect(parseModelTableId("models")).toBeUndefined();
    expect(parseModelTableId("model")).toBeUndefined();
    expect(parseModelTableId("ui")).toBeUndefined();
  });
});

describe("listUserLocalModels", () => {
  it("returns user tables and skips managed ones", () => {
    const toml = `${BASE}
[model.local-llama]
model = "llama-3.1-70b"
name = "Local Llama"
base_url = "http://localhost:8080/v1"
api_key = "sk-secret"
context_window = 32000

[model.company-hosted]
model = "enc-id"
managed = true
name = "Hosted"
`;
    expect(listUserLocalModels(toml)).toEqual([
      {
        id: "local-llama",
        model: "llama-3.1-70b",
        name: "Local Llama",
        description: undefined,
        baseUrl: "http://localhost:8080/v1",
        envKey: undefined,
        hasApiKey: true,
        contextWindow: 32000,
      },
    ]);
  });
});

describe("upsertUserLocalModel", () => {
  it("appends a new table without disturbing [ui] or comments", () => {
    const next = upsertUserLocalModel(BASE, {
      id: "local-llama",
      model: "llama-3.1-70b",
      name: "Local Llama",
      baseUrl: "http://127.0.0.1:11434/v1",
      envKey: "OLLAMA_API_KEY",
    });
    expect(next).toContain("# Atlas global configuration");
    expect(next).toMatch(/\[ui\][\s\S]*permission_mode = "ask"/);
    expect(next).toContain("[model.local-llama]");
    expect(next).toContain('model = "llama-3.1-70b"');
    expect(next).toContain('base_url = "http://127.0.0.1:11434/v1"');
    expect(next).toContain('env_key = "OLLAMA_API_KEY"');
    expect(next).not.toContain("api_key");
  });

  it("replaces an existing user table and keeps other model tables", () => {
    const toml = `${BASE}
[model.keep-me]
name = "Keep"

[model.local-llama]
model = "old"
api_key = "sk-old"
`;
    const next = upsertUserLocalModel(toml, {
      id: "local-llama",
      model: "new-id",
      name: "Renamed",
    });
    expect(next).toContain("[model.keep-me]");
    expect(next).toContain('name = "Keep"');
    expect(next).toContain('model = "new-id"');
    expect(next).toContain('name = "Renamed"');
    expect(next).toContain('api_key = "sk-old"');
    expect(next.match(/\[model\.local-llama\]/g)?.length).toBe(1);
  });

  it("clears api_key when the draft sends an empty string", () => {
    const toml = `[model.local-llama]\napi_key = "sk-old"\nmodel = "x"\n`;
    const next = upsertUserLocalModel(toml, { id: "local-llama", model: "x", apiKey: "" });
    expect(next).not.toContain("api_key");
  });

  it("refuses to overwrite a managed table", () => {
    const toml = `[model.hosted]\nmanaged = true\nmodel = "enc"\n`;
    expect(() => upsertUserLocalModel(toml, { id: "hosted", name: "nope" })).toThrow(LocalModelError);
    expect(upsertUserLocalModel(toml, { id: "other", name: "ok" })).toContain("managed = true");
  });

  it("rejects an invalid id", () => {
    expect(() => upsertUserLocalModel("", { id: "bad id" })).toThrow(/Invalid model id/);
  });
});

describe("removeUserLocalModel", () => {
  it("drops the named user table and leaves the rest", () => {
    const toml = `${BASE}
[model.keep-me]
name = "Keep"

[model.gone]
name = "Gone"
`;
    const next = removeUserLocalModel(toml, "gone");
    expect(next).toContain("[model.keep-me]");
    expect(next).not.toContain("[model.gone]");
    expect(next).toContain("[ui]");
  });

  it("refuses to remove a managed table", () => {
    const toml = `[model.hosted]\nmanaged = true\n`;
    expect(() => removeUserLocalModel(toml, "hosted")).toThrow(/managed/);
  });
});
