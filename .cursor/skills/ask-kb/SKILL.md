---
name: ask-kb
description: >
  自然语言查询知识库。将用户问题翻译为 kb-server 的 kb.search / kb.query / kb.traverse
  工具调用，检索实体、文档片段和关联关系，返回结构化的 Markdown 结果。
  当用户问「XXX是什么」「查一下YYY」「ZZZ和WWW的关系」或运行 /ask-kb 时触发。
---

# ask-kb

自然语言查询本工程的知识库（knowledge-base）。通过调用 kb-server 暴露的
`POST /api/v1/query` 接口，使用 SSE 流式返回结果。

## 何时触发

用户用自然语言询问以下任意主题时：
- 概念定义：「什么是 XXX」「XXX 是什么」
- 实体搜索：「查一下 YYY」「搜索 ZZZ」
- 关系/影响：「YYY 和 ZZZ 的关系」「改 XXX 会影响哪些表/服务」
- 文档溯源：「XXX 来自哪篇文档」「XXX 的原文出处」
- 显式调用：用户输入 `/ask-kb` 后接问题

## 1. 确定 kb-server 地址

本工程 kb-server 部署在：

```
http://10.218.220.236:8089
```

按优先级选择 URL：
1. 如果用户显式指定了其他地址（如本地调试 `http://localhost:8080`），使用用户指定的地址。
2. 否则固定使用本工程默认地址 `http://10.218.220.236:8089`。
3. 如果连默认地址也无法访问，可读取 `kb-server/config.yaml` 的 `server.port` 字段作为本地回退。

## 2. 选择查询工具

根据用户意图选择 `tool` 字段：

### kb.search（默认）

用于关键词全文搜索实体和文档片段。

参数 `tools.SearchParams`：
```json
{
  "query": "用户关键词",
  "type": "可选，限定类型，例如 kb:DataEntity / kb:Service / pcp:FeignAPI",
  "limit": 10
}
```

示例请求：
```bash
curl -N -X POST http://{host}/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "kb.search",
    "params": {
      "query": "门店库存",
      "limit": 10
    }
  }'
```

### kb.traverse

用于查关联、影响分析、上下游追踪。

参数 `tools.TraverseParams`：
```json
{
  "uri": "实体的完整 URI 或 QName，例如 http://yumchina.com/pcp/instance#svc-inventory",
  "relation": "可选，限定关系，例如 kb:dependsOn / kb:produces / kb:consumes",
  "direction": "可选，out / in / both，默认 both",
  "depth": 2,
  "limit": 50
}
```

示例请求：
```bash
curl -N -X POST http://{host}/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "kb.traverse",
    "params": {
      "uri": "inst:svc-inventory",
      "direction": "both",
      "depth": 2,
      "limit": 50
    }
  }'
```

### kb.query

用于直接执行 SPARQL。支持两种模式：

1. **结构化查询（默认）**：按类型和属性过滤实体。
2. **原始 SPARQL**：传入 `sparql` 字段，直接执行 `SELECT` 查询并以 Markdown 表格返回结果。

> **选型提示**：当用户问「XXX 来自哪篇文档」「原文出处」「GitLab 链接」时，优先用原始 SPARQL 查询 `kb:Document` 的 `kb:docPath`；结构化查询不返回该属性。

参数 `tools.QueryParams`（结构化）：
```json
{
  "type": "kb:BusinessService",
  "filters": {"severity": "CRITICAL"},
  "limit": 20
}
```

参数 `tools.QueryParams`（原始 SPARQL）：
```json
{
  "sparql": "SELECT ?s ?p ?o WHERE { ?s a kb:Document . ?s kb:docPath ?o } LIMIT 10"
}
```

> 注意：
> - 原始 SPARQL 仅支持 `SELECT` 读查询，禁止 `INSERT/DELETE/UPDATE/LOAD/CLEAR/DROP` 等写操作。
> - 如果用到 `kb:` 等前缀，必须在查询里显式写 `PREFIX` 声明（kb-server 不会自动补前缀）。
> - GitLab 模式下的 `kb:docPath` 值是完整 GitLab blob URL 路径（含 `/-/blob/master/`），前面拼上 `https://gitlab.imyai.cn/` 就是可点击的原文链接。

#### 常用场景：查某篇文档的 GitLab 原文出处

```bash
curl -N -X POST http://{host}/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "kb.query",
    "params": {
      "sparql": "PREFIX kb: <http://yumchina.com/kb/ontology#>\\nSELECT ?doc ?name ?docPath\\nWHERE {\\n  ?doc a kb:Document ;\\n       kb:name ?name ;\\n       kb:docPath ?docPath .\\n  FILTER(CONTAINS(LCASE(STR(?name)), \"order control tower\"))\\n}\\nLIMIT 10"
    }
  }'
```

返回的 Markdown 表格里，`docPath` 列就是完整 GitLab 路径，例如：
`yum/boh/knowledge-base/system-oh-1014-docs/tower/-/blob/master/00-overview/order-control-tower-requirements-analysis-2026-03-20.md`

#### 通用 SELECT 查询示例
```bash
curl -N -X POST http://{host}/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "kb.query",
    "params": {
      "sparql": "SELECT ?doc ?name ?docPath WHERE { ?doc a kb:Document ; kb:name ?name ; kb:docPath ?docPath } LIMIT 10"
    }
  }'
```

## 3. 执行查询并解析结果

接口返回 SSE 流，事件类型包括：
- `start` — 开始
- `result` — 一段 Markdown 结果（可能多个）
- `error` — 错误信息
- `done` — 完成，附带 `total` 和 `elapsedMs`

使用 curl 时加上 `-N`（no-buffer）确保实时输出：
```bash
curl -N -X POST http://{host}/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{...}' 2>/dev/null
```

将所有 `result` 事件的 `chunk` 字段按顺序拼接，就是完整 Markdown 答案。

## 4. 回答用户

- 如果搜索到结果：把拼接后的 Markdown 摘要展示给用户，并说明来源类型（实体/文档片段/关系）。
- 如果结果为空：提示用户尝试更短/更通用的关键词，或改用 `kb.traverse` 做关联查询。
- 如果报错：展示错误内容，并判断是地址不通、参数错误还是 Fuseki 查询失败。

## 5. 多轮查询策略

复杂问题分两步：
1. 先用 `kb.search` 找到目标实体 URI。
2. 再用 `kb.traverse` 查该实体的关联关系。

例如「改库存服务会影响哪些表」：
1. `kb.search` 找 `库存服务` → 得到 `inst:svc-inventory`
2. `kb.traverse` 从 `inst:svc-inventory` 出发查 `dependsOn` / `consumes` / `produces`

## 限制

- 不要假设知识库一定包含答案；返回空时明确告知。
- 不要替用户构造可能修改数据的 SPARQL（INSERT/DELETE）。
- 调用失败时优先检查 kb-server 是否启动、Fuseki 是否可连。
