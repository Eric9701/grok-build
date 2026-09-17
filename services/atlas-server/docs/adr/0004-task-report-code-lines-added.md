# Task Report 只统计代码新增行

## Status

Accepted (2026-09-15)

## Context

Admin 已能按路径把产物分成 `code` / `doc` / `other`，但汇总只有文件个数。需要一条「写了多少代码」的指标时，有三种常见口径：净增行、新增行、写后全文行数；文档要不要算也是分叉。

## Decision

只统计 **代码的新增行**（`codeLinesAdded`）：

- 行 = 成功的 `write` / `edit` / `apply_patch` 上 `edit.lines` 的 Insert 次数（与 `line_diff` 同一计数）；删除不计。工具成功时算一次并写入输出，Task Report 只读该计数，不再对拍差分。
- 「代码」沿用现有产物 `kind=code`（含 json/yaml/html/css 等），`doc` / `other` 不计行。
- 同一文件多次编辑累加。
- bash 改文件仍不进产物，因此也不进行数。
- CLI 上报每路径 `artifactLinesAdded`；服务端分类后求和并落 `code_lines_added`，供单笔详情和 Admin 汇总。旧客户端不报该字段时记 0。

不采用净增行（编辑会冲掉新增）、不采用全文行数（覆盖写入会把旧文件算进去）。

## Consequences

- 新建文件的全文都算新增；覆盖写入只算相对旧内容插入的行。
- 与 hunk-tracker 的 accepted/rejected 行数不是同一条管道，不要混读。
- 改分类规则会同时影响文件数 kind 与代码行。
