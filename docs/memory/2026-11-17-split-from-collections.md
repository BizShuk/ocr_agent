# 2026-11-17 從 collections 拆出 ocr_agent

## 背景

收據 pipeline 原本是 `~/projects/default` (collections archive) 內的 `collections receipt` CLI。
它只有一組 `model.*` 設定：整條 pipeline 共用一個 provider，`receipt.root` 固定 `tmp/`、`general/tmp/`、`medical/tmp/` 的佈局，
`model.base_url` 會被所有 provider 共用。

## 決策

- 工具搬到 `~/projects/tools/ocr_agent`，資料留在 `~/projects/default/pkg/receipts/`。
- 設定改為每個 domain 一個 block (`provider`、`model`、`source`、`target`…)，例如 general 可走 minimax、medical 留在本機 ollama。
- classify 不設 target，直接路由到下游 domain 的 source。
- 舊的 UOB 專用 analyze prompt 移除；`analyze <domain>` 改用該 domain 自己的 prompt 印出原始回覆。
- 不保留 `collections` 指令或舊 config key 的相容層。

## 歷史文件

- `docs/specs/2026-07-18-receipt-pipeline-hardening.md`、`docs/memory/2026-07-18-receipt-pipeline-hardening-plan.md`：
  Python 版 → Go 版時期的 hardening 設計與計畫，保留作為歸檔規則的來源紀錄。
