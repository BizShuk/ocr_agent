# 術語表 (Terminology)

| 術語 | 定義 |
| --- | --- |
| `task` | `tasks[]` 的一個 object：`name`、`description`、`type`、`provider`、`model`、`timeout`、`source`、`targets` |
| `task type` | `classify`（遞迴掃描、搬原檔）或 `date`（flat 掃描、需 `date`、搬原檔 + sidecar 到日期資料夾）；一個 type 一個 subcommand |
| `mv` | 每圖一次 model call 時附上的 agentSDK tool；參數 `target`（enum）與 `metadata` |
| `metadata` | `mv` 的 JSON 參數；date task 寫成 sidecar |
| `provider` | agentsdk registry 中的模型供應商名稱，例如 `ollama`（本機）、`minimax`（遠端） |
| `source` | task 讀取影像的 folder |
| `target` | task 提供給模型挑選的目的地：`name`、`description`、`path`；以 `mv` 的 `target` enum 呈現 |
| `placeholder` | target `path` 內的 `{yyyy}` / `{mm}`，由 date task 的 `date` 填入 |
| `sidecar` | 與影像同 stem 的 `.json`，內含 date task 的 `metadata` 與 `original_filename` |
| `dry-run` | 預設模式：呼叫模型並驗證，但不建資料夾、不搬檔、不寫 sidecar |
| `MC` | Medical Certificate，病假證明 |
| `ROC year` | 民國紀年；年份 ≤ 1911 時加 1911 換算為西元 |
| `UEN` | 新加坡商家 / 診所登記號 |
| `MCR` | 新加坡醫師執業登記號 (Medical Council Registration) |
