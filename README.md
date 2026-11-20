# ocr_agent — 文件影像 OCR 歸檔代理 (Document OCR Agent)

> 用視覺模型 (vision model) 依設定中的 task 判讀文件影像，由模型挑選目的資料夾並搬移。
> Reads document images per configured task and moves each into a model-chosen folder.

## 1. 定位 (Purpose)

`ocr_agent` 是取代 `~/projects/default` 內 `collections receipt` 的獨立 CLI。資料仍放在
`~/projects/default/pkg/receipts/`，工具本身不再與 archive repo 綁在一起。

設定是一份 `tasks` 清單，每個 object 是一個完整的 task：`name`、`description`（要模型做什麼）、
`type`（搬什麼）、`provider` / `model` / `timeout`（問誰）、`source`（讀哪裡）、`targets`（可以搬去哪裡）。

## 2. Task 類型 (Task Types)

| `type` / subcommand | 讀 `source` | `metadata` 要求 | 搬到所選 target 的檔案 |
| --- | --- | --- | --- |
| `classify` | 遞迴 | 自由的 JSON object，只記在 log | 原檔 |
| `date` | flat | 必須有 `date`（`YYYY-MM-DD`，民國年 +1911，不可未來） | 原檔 + `metadata` 當 `.json` sidecar，進 target 的 `{yyyy}/{mm}` |

每張影像一次模型呼叫，用該 task 的 model：送出影像、組好的 prompt 與 agentSDK 的 `mv` tool。
prompt 由 task `description` + target 清單 + 該 type 的結果格式組成；模型以一次
`mv({"target": "<name>", "metadata": {...}})` 回答。`target` 是 target name 的 enum，模型只能挑名稱，
搬哪個檔與實際路徑由程式決定。模型沒有呼叫 `mv` 時檔案留在原處，log 印出模型的說明。

同一 type 可以有多個 task。預設 seed 有一個 classify 與三個 date task：

```text
inbox   (classify): receipts/tmp --> general/tmp | medical/tmp | special/tmp
general (date)    : general/tmp  --> general/{yyyy}/{mm}  (+ sidecar)
medical (date)    : medical/tmp  --> medical/{yyyy}/{mm}  (+ sidecar)
special (date)    : special/tmp  --> special/{yyyy}/{mm}  (+ sidecar)
```

## 3. 使用方式 (Usage)

所有指令預設 `dry-run`：會呼叫模型並解析目的地，但不建資料夾、不搬檔、不寫 sidecar；加 `--apply` 才執行。
進度與摘要寫 stdout，單張失敗與最終錯誤寫 stderr。

```bash
ocr_agent run --apply                     # 遍歷 tasks[], 依設定順序跑全部

ocr_agent classify --apply                # 只有一個 classify task 時可省略 --source
ocr_agent date --source ~/projects/default/pkg/receipts/medical/tmp --apply
ocr_agent date --source ~/projects/default/pkg/receipts/general/tmp --file ~/Downloads/IMG_1260.JPG   # dry-run 單張
```

type subcommand 不遍歷 tasks[]：它以 `--source` 找出該 type 中 `source` 相符的那一個 task，直接用它的設定處理。
該 type 只有一個 task 時可省略 `--source`。`--file` 改處理單張影像（仍用該 task 的設定）。model 與 provider 只在設定檔修改。
credential 只從環境變數或 `~/.config/ocr_agent/.env` 取得。

## 4. 設定 (Configuration)

設定檔為 `~/.config/ocr_agent/settings.json`，首次以 `ocr_agent config default`（或 `npm run run:setup`）
從內嵌的 [`settings.example.json`](config/example/settings.example.json) 產生（含三個 task 的完整 description）。
`ocr_agent config --source` 顯示每個值的來源。

```json
{
  "tasks": [
    { "name": "general", "type": "date",
      "description": "Analyze this general receipt image. ... Put these fields in metadata: {\"date\": \"YYYY-MM-DD\", \"merchant\": ...}",
      "provider": "minimax", "model": "MiniMax-M3", "timeout": "10m",
      "source": "~/projects/default/pkg/receipts/general/tmp",
      "targets": [ { "name": "general_archive", "description": "Extracted ordinary receipts ...", "path": "~/projects/default/pkg/receipts/general/{yyyy}/{mm}" } ] }
  ]
}
```

| Key `tasks[].*` | 必要 | 說明 |
| --- | --- | --- |
| `name` | 是 | task 名稱，清單內唯一；用於 log |
| `description` | 是 | 要模型判讀或擷取什麼（欄位、規則）；程式會再附上 target 清單與回答格式 |
| `type` | 是 | `classify` 或 `date`；決定 subcommand、掃描方式與搬什麼 |
| `provider` | 是 | agentsdk registry 的 provider 名稱：`ollama`、`minimax`、`anthropic` … |
| `model` | 是 | 該 provider 的 vision model，必須支援 tool call |
| `timeout` | 否 | 單次 HTTP timeout，預設 `10m` |
| `source` | 是 | 讀取影像的 folder；`~` 會展開；同 type 內唯一，subcommand 的 `--source` 以它選 task |
| `targets` | 是 | 至少一個；見下表 |
| `base_url` | 否 | endpoint 覆寫；留空時用 provider 自身的環境變數（`OPENAI_BASE_URL`、`MINIMAX_BASE_URL`）或預設 |

| Key `tasks[].targets[].*` | 必要 | 說明 |
| --- | --- | --- |
| `name` | 是 | 模型在 `mv` 呼叫中使用的名稱，task 內唯一 |
| `description` | 是 | 什麼文件該放這裡；模型只憑它挑選 |
| `path` | 是 | 目的 folder；`~` 會展開；`{yyyy}` / `{mm}` 由 `date` 填入，classify task 選到這種 target 時該檔失敗 |

限制：classify task 的 target 不可位於自己的 `source` 之內（遞迴掃描會重複分類）；選到檔案目前所在的 folder 時該檔失敗。
`tasks` 是陣列，只能在設定檔修改；遠端 provider 的 credential 走該 provider 的環境變數，例如 `MINIMAX_API_KEY`。

## 5. 歸檔規則 (Filing Rules)

- 檔名保留原名；同名時加 `_2`、`_3`；image 與同 stem 的 `.json` sidecar 都不覆寫。
- `date` task 的 sidecar 是 `mv` 的 `metadata`（`date` 已正規化成西元），另加 `original_filename`、`renamed_to`。
- 任一檔案失敗時該 task 以 exit code `1` 結束，但會繼續處理其他檔案與其他 task。
- sidecar 寫入失敗時，影像會搬回原 `source`。

## 6. 排程 (Scheduling)

pm2 排程由資料 repo `~/projects/default/ecosystem.config.js` 擁有，每個 task 一個 one-shot cron（`ocr_agent <type> --source <dir> --apply`），或一個 `ocr_agent run --apply`：
`ocr-classify` 05:00、`ocr-general` 06:20、`ocr-medical` 06:40，log 寫到 `~/.config/ocr_agent/logs/`。

## 7. 隱私與安全 (Privacy & Safety)

- Provider 是每個 task 的隱私分界：`ollama` 走本機 `localhost:11434`，影像不出本機；
  `minimax` 等遠端 provider `每張影像都會上傳`。選用遠端 provider 是使用者的明確決定，不由工具或 agent 代選。
- `source` 與 target `path` 內是不可任意刪除的真實紀錄；未經明確要求不執行 `--apply`。
- 不把 NRIC、完整卡號等敏感資訊放進檔名。

## 8. 開發 (Development)

```bash
npm run test     # go test ./...
npm run lint     # gofmt + go vet
npm run install  # go install . -> ocr_agent on PATH
```

技術結構見 [AGENTS.md](AGENTS.md)。
