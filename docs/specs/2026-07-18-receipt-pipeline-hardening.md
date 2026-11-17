# Receipt Pipeline Hardening Design

## 目標

修復 `scripts/receipt/` 的檔案覆寫、模型輸出驗證、醫療欄位契約、sidecar 寫入、CLI 狀態、分析工具與 pm2 排程問題，同時保留既有歸檔檔名，不批次改名或處理真實收據。

## Canonical 檔名與目錄

自動處理後的收據使用下列格式：

```text
receipts/general/<YYYY-MM>/<YYYY-MM-DD>_<merchant>_<currency><amount>.<ext>
receipts/medical/<YYYY-MM>/<YYYY-MM-DD>_<clinic>_<currency><amount>.<ext>
receipts/medical/<YYYY-MM>/<YYYY-MM-DD>_<clinic>_mc.<ext>
```

- `merchant` 與 `clinic` 使用小寫底線格式。
- `currency` 必須是三個小寫 ASCII 字母。
- `amount` 固定兩位小數。
- 此規則只套用於新處理的檔案；不遷移或重新命名既有 archive。

## 資料流

```text
receipts/tmp
    -> classify_receipts.py
    -> receipts/general/tmp 或 receipts/medical/tmp
    -> process_general_receipts.py 或 process_medical_receipts.py
    -> receipts/<category>/<YYYY-MM>/<image + JSON sidecar>
```

新增 `scripts/receipt/process_receipts.py` 作為完整手動流程入口，依序執行分類、一般收據、醫療收據三階段。即使某一階段回報部分檔案錯誤，後續階段仍會執行；pipeline 最終以非零狀態回報任一階段失敗。pm2 只排程分類 stage，Stage 2 保持手動執行。

## 模型回應驗證

所有 Ollama JSON 回應在組路徑或搬檔前必須驗證：

- 最外層必須是 JSON object。
- `category` 必須符合目前處理器。
- `date` 必須是有效且不晚於執行當日的 `YYYY-MM-DD`。
- 名稱正規化後不可為空。
- `currency` 必須符合 `[a-z]{3}`。
- `total_amount` 必須是有限、非負且最多兩位小數的數值。
- 醫療回應的 `is_medical_certificate` 必須是 JSON boolean。
- 非 MC 醫療收據必須有 `currency` 與 `total_amount`；MC 不需要這兩欄。
- 產生的目的路徑解析後必須留在對應 category 目錄內。

無效模型資料視為該檔案的 validation error：不搬檔、繼續下一檔，整批最終回傳非零狀態。

## 檔案安全

分類階段若目的檔名已存在，使用 `_2`、`_3` 後綴，不覆寫既有檔案。

歸檔階段同時為圖片與 JSON sidecar 選擇可用 stem。寫入順序為：

1. 在目的目錄寫入並 flush 暫存 JSON。
2. 搬移圖片到目的位置。
3. 原子 rename 暫存 JSON 成正式 sidecar。
4. 若步驟 2 或 3 失敗，清除暫存檔；步驟 3 失敗時嘗試將圖片搬回原位置。

此流程避免一般 I/O 錯誤留下無法重試的半完成狀態；兩個檔案的目的名稱都納入 collision 檢查。

## CLI 與錯誤處理

- Dry-run 不建立目錄、不搬檔、不寫 sidecar。
- `--file` 必須存在、為一般檔案且副檔名受支援。
- 每一檔的讀取、HEIC 轉換、Ollama、validation 與寫檔錯誤均隔離處理，不中止其餘檔案。
- 有任何 error 時 exit code 為 `1`；只有 processed/skipped 且無 error 時為 `0`。
- Validation error 列入 error，不列入成功處理。

## Analyzer

`analyze_receipt.py` 使用正確 project root，並透過 `urllib` request body 呼叫 Ollama，不把 base64 圖片放入命令列參數。HEIC 暫存檔無論成功或失敗都清除。

## pm2

`ecosystem.config.js` 只註冊 receipt classifier 排程：

```text
name: classify-receipts
script: ./scripts/receipt/classify_receipts.py
args: --apply
cron: 0 5 * * *
autorestart: false
```

此排程只把 `receipts/tmp/` 路由到 general/medical temporary folders，不執行 Stage 2 歸檔。

## 測試

使用 Python standard library `unittest`，以 temporary directory 與 mock Ollama response 驗證：

- 分類同名 collision 不覆寫。
- Dry-run 零寫入。
- 日期、category、currency、amount 與路徑 containment。
- 醫療 MC 與非 MC 條件欄位。
- 金額固定兩位小數。
- 圖片與 sidecar collision、成功寫入及失敗 rollback。
- 單檔失敗不阻擋下一檔，且整批 exit code 非零。
- Analyzer 使用正確 root 與 HTTP body。
- Pipeline 三階段順序及 aggregated exit code。

測試不得讀取、搬移或傳送 `receipts/` 內的真實圖片。

## 非目標

- 不批次修正既有錯誤日期或檔名。
- 不搬移或重新產生既有 sidecar。
- 不變更 Ollama model 或 prompt 的辨識策略。
- 不執行 `--apply` 處理真實收據。
- 不操作目前運行中的 pm2 daemon。
