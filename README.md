# ocr_agent — 文件影像 OCR 歸檔代理 (Document OCR Agent)

> 用視覺模型 (vision model) 判讀文件影像，依 domain 分類、擷取 metadata、重新命名並按月歸檔。
> Classifies document images with a vision model, extracts metadata, renames and archives them by month.

## 1. 定位 (Purpose)

`ocr_agent` 是取代 `~/projects/default` 內 `collections receipt` 的獨立 CLI。資料仍放在
`~/projects/default/pkg/receipts/`，工具本身不再與 archive repo 綁在一起。

每個 `domain` 都有自己的設定：用哪個 `provider`（本機 `ollama` 或遠端 `minimax` 等）、哪個 model、
從哪個 `source` folder 讀、歸檔到哪個 `target` path。

## 2. Domains

| Domain | 輸入 | 輸出 | 用途 |
| --- | --- | --- | --- |
| `classify` | `domain.classify.source`（遞迴） | `domain.general.source` 或 `domain.medical.source` | 判斷影像是 `general_receipt`、`medical_receipt` 或 `non_receipt` |
| `general` | `domain.general.source`（flat） | `domain.general.target/<YYYY>/<MM>/` | 一般收據：商家、金額、品項 |
| `medical` | `domain.medical.source`（flat） | `domain.medical.target/<YYYY>/<MM>/` | 醫療收據與 MC：診所、病患、醫師、病假日期 |

`classify` 沒有 `target`：分類結果直接落到下游 domain 的 `source`，兩者只有一個定義來源。
`non_receipt` 留在原處不動。

```text
domain.classify.source
    -> ocr_agent classify
    -> domain.general.source | domain.medical.source
    -> ocr_agent general | ocr_agent medical
    -> <target>/<YYYY>/<MM>/<image + JSON sidecar>
```

## 3. 使用方式 (Usage)

所有會改檔的指令預設 `dry-run`：會呼叫模型與驗證，但不建資料夾、不搬檔、不寫 sidecar；加 `--apply` 才執行。

```bash
ocr_agent process                 # 三個 domain 依序 dry-run
ocr_agent process --apply         # 依序執行 classify -> general -> medical

ocr_agent classify --apply
ocr_agent general --file ~/Downloads/IMG_1260.JPG
ocr_agent medical --provider ollama --model qwen2.5vl:7b   # 單次覆寫該 domain 的 model

ocr_agent analyze general --file ~/Downloads/IMG_1260.JPG  # 印出原始模型回覆, 不動任何檔案
```

`--provider` / `--model` 只影響該次執行；`process` 一律使用設定檔。沒有 `--api-key` flag：argv 會被 `ps` 讀到，
credential 只從環境變數或 `~/.config/ocr_agent/.env` 取得。

## 4. 設定 (Configuration)

設定檔為 `~/.config/ocr_agent/settings.json`，首次以 `ocr_agent config default`（或 `npm run run:setup`）
從內嵌的 [`settings.example.json`](settings.example.json) 產生。`ocr_agent config --source` 顯示每個值的來源。

```json
{
  "domain": {
    "classify": { "provider": "ollama",  "model": "qwen2.5vl:3b", "source": "~/projects/default/pkg/receipts/tmp" },
    "general":  { "provider": "minimax", "model": "MiniMax-M3",
                  "source": "~/projects/default/pkg/receipts/general/tmp",
                  "target": "~/projects/default/pkg/receipts/general" },
    "medical":  { "provider": "ollama",  "model": "z-uo/qwen2.5vl_tools:7b",
                  "source": "~/projects/default/pkg/receipts/medical/tmp",
                  "target": "~/projects/default/pkg/receipts/medical" }
  }
}
```

| Key `domain.<name>.*` | 必要 | 說明 |
| --- | --- | --- |
| `provider` | 是 | agentsdk registry 的 provider 名稱：`ollama`、`minimax`、`anthropic` … |
| `model` | 是 | 該 provider 的 vision model |
| `source` | 是 | 讀取影像的 folder；`~` 會展開 |
| `target` | general / medical | 歸檔根目錄；classify 不使用 |
| `base_url` | 否 | endpoint 覆寫；留空時用 provider 自身的環境變數（`OPENAI_BASE_URL`、`MINIMAX_BASE_URL`）或預設 |
| `timeout` | 否 | 單張影像的 HTTP timeout，預設 `10m` |

OS 環境變數覆寫任一 key：`DOMAIN_GENERAL_PROVIDER=ollama` 覆寫 `domain.general.provider`。
遠端 provider 的 credential 走該 provider 的環境變數，例如 `MINIMAX_API_KEY`。

## 5. 歸檔規則 (Filing Rules)

```text
<general target>/<YYYY>/<MM>/<YYYY-MM-DD>_<merchant>_<currency><amount>.<ext>
<medical target>/<YYYY>/<MM>/<YYYY-MM-DD>_<clinic>_<currency><amount>.<ext>
<medical target>/<YYYY>/<MM>/<YYYY-MM-DD>_<clinic>_mc.<ext>
```

- `merchant`、`clinic` 為小寫底線；`currency` 為三個小寫字母；`amount` 固定兩位小數。
- 同名時加 `_2`、`_3`；image 與同 stem 的 `.json` sidecar 都不覆寫。
- 模型回覆必須是 JSON object；date 必須是有效且非未來的 `YYYY-MM-DD`；民國年 (ROC) 加 1911 換算。
- 任一檔案失敗時該 domain 以 exit code `1` 結束，但會繼續處理其他檔案。
- sidecar 寫入失敗時，影像會搬回原 `source`。

## 6. 排程 (Scheduling)

pm2 排程由資料 repo `~/projects/default/ecosystem.config.js` 擁有，每個 domain 一個 one-shot cron：
`ocr-classify` 05:00、`ocr-general` 06:20、`ocr-medical` 06:40，log 寫到 `~/.config/ocr_agent/logs/`。

## 7. 隱私與安全 (Privacy & Safety)

- Provider 是每個 domain 的隱私分界：`ollama` 走本機 `localhost:11434`，影像不出本機；
  `minimax` 等遠端 provider `每張影像都會上傳`。選用遠端 provider 是使用者的明確決定，不由工具或 agent 代選。
- `source` / `target` 內是不可任意刪除的真實紀錄；未經明確要求不執行 `--apply`。
- 不把 NRIC、完整卡號等敏感資訊放進檔名。

## 8. 開發 (Development)

```bash
npm run test     # go test ./...
npm run lint     # gofmt + go vet
npm run deploy   # go install . -> ocr_agent on PATH
```

技術結構見 [AGENTS.md](AGENTS.md)。
