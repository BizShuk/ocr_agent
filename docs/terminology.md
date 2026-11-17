# 術語表 (Terminology)

| 術語 | 定義 |
| --- | --- |
| `domain` | 一個處理階段及其設定 block `domain.<name>`：`classify`、`general`、`medical` |
| `provider` | agentsdk registry 中的模型供應商名稱，例如 `ollama`（本機）、`minimax`（遠端） |
| `source` | domain 讀取影像的 folder；classify 遞迴掃描，general / medical 只看第一層 |
| `target` | general / medical 的歸檔根目錄，底下為 `<YYYY>/<MM>/` |
| `route` | classify 把影像搬進下游 domain `source` 的動作 |
| `category` | classify 的判定，封閉集合：`general_receipt`、`medical_receipt`、`non_receipt` |
| `sidecar` | 與歸檔影像同 stem 的 `.json`，內含模型擷取的 metadata 與 `original_filename` |
| `dry-run` | 預設模式：呼叫模型並驗證，但不建資料夾、不搬檔、不寫 sidecar |
| `MC` | Medical Certificate，病假證明；檔名以 `_mc` 結尾、不含金額 |
| `ROC year` | 民國紀年；年份 ≤ 1911 時加 1911 換算為西元 |
| `UEN` | 新加坡商家 / 診所登記號 |
| `MCR` | 新加坡醫師執業登記號 (Medical Council Registration) |
