# ocr_agent — 技術脈絡 (Technical Context)

本檔記錄結構、分層與關鍵決策。功能、domain 與 CLI 用法由 [README.md](README.md) 單一擁有。

## 目錄結構 (Directory Layout)

```tree
ocr_agent/
├── main.go                 # config.Default + 內嵌 config/example/settings.example.json -> cmd.ExecuteContext
├── config/example/settings.example.json  # `ocr_agent config default` 的 seed (全 minimax / MiniMax-M3)
├── cmd/                    # cobra: 旗標、domain 設定解析、輸出; 不做歸檔決策
│   ├── root.go             # RootCmd, 掛 gosdk ConfigCmd 與各 domain 命令
│   ├── classify.go         # classify domain, 辨識影像類型並路由到下游 domain 的 source
│   ├── process.go          # 三個 domain 依序執行
│   ├── analyze.go          # 原始模型回覆, prompt 調校用
│   ├── internal/           # cmd 與子套件共用的設定與檔案列舉 helper
│   │   ├── domain.go       # domain.<name>.* 讀取、--provider/--model 覆寫、provider registry 建構
│   │   └── files.go        # source 掃描與 --file 驗證
│   └── receipt/            # 收據類 domain 命令
│       ├── extract.go      # general / medical 共用的 extract runner
│       ├── general.go      # general 命令
│       └── medical.go      # medical 命令
├── svc/                    # 領域邏輯, 不 import cobra / viper
│   ├── imagefile/          # 影像載入 (HEIC -> JPEG via sips) 與列舉
│   ├── vision/             # prompt + image -> 模型回覆; JSON 擷取
│   ├── classify/           # Category 封閉集合、classify prompt 與解析
│   ├── receipt/            # general / medical prompt、metadata 驗證、命名、Processor
│   └── filing/             # 不覆寫的目的地挑選、Move、image + sidecar 原子提交
├── scripts/run.sh          # run:setup: 建 ~/.config/ocr_agent/{data,logs}、tmp/config symlink、補齊 seed key
└── docs/                   # terminology、memory、specs
```

相依方向：`cmd -> svc/receipt -> svc/{classify,filing,vision,imagefile}`，`svc/classify -> svc/{vision,imagefile}`。
`svc/` 只拿到已解析好的值 (provider、路徑)，對 viper 一無所知。

## 關鍵決策 (Key Decisions)

### Per-domain 設定

每個 domain 一個 `domain.<name>` block，欄位相同 (`provider`、`model`、`base_url`、`timeout`、`source`、`target`)。
這取代了舊版的單一 `model.provider` + `model.classify_model` / `model.process_model` + `receipt.root`：
舊版只能整條 pipeline 用同一個 provider，而且 `model.base_url` 會被所有 provider 共用 (切到 minimax 會拿到 ollama 的 endpoint 回 404)。
現在 `base_url` 屬於 domain，各自留空即走各 adapter 自己的預設。

`LoadDomain` 只強制 `source`；`provider`/`model` 在 `NewProvider` 檢查，`target` 在 `RequireTarget` 檢查。
原因是 classify 讀 general / medical 時只需要它們的 `source`，不該因為下游 model 沒設而失敗。

### classify 沒有 target

classify 的輸出就是下游 domain 的 `source`，所以不另設 `target`，避免兩處設定漂移。
`classifyRoutes` 拒絕「下游 source 位於 classify source 之內」：classify 遞迴掃描，否則每次執行都會把已分類的檔案再分類一次。

### Provider 由 registry 名稱決定

`cmd/domain.go` blank-import `agentsdk/provider/all`，以 `provider.New(name, …)` 建構 adapter。換 vendor 是改設定，不是改 code。
`LookupEnv` 先查 viper 再查 OS env，所以 `~/.config/ocr_agent/.env` 內的 `MINIMAX_API_KEY` 也有效。

### 沒有 code 內建預設值

provider / model / 路徑只來自設定 (seed 由 `config default` 寫入)，缺值時錯誤訊息會指出完整 key。
唯一的 code 預設是 `timeout` 的 `10m`。

## 測試 (Testing)

- `cmd/cmd_test.go` 在 agentsdk registry 註冊一個 `stub` provider，以 `viper.Set` 建 temp 目錄的三個 domain，
  驗證路由、每個 domain 各建一個 provider、覆寫、與缺 key 錯誤。
- `svc/*` 測試不需要模型或真實影像；HEIC 測試會呼叫 macOS `sips`。
- Verification 不得讀取、辨識或搬移真實收據。

## 陷阱 (Pitfalls)

- `go mod tidy` 在 `main.go` 尚未 import gosdk 時會把它移除，之後再加回會解析到舊版 (曾降到 v1.3.5)；改完 import 後確認 `go.mod` 的版本。
- package-level flag 變數在同一 process 內跨 `Execute()` 保留值；測試直接呼叫 `runClassify` / `runExtract` 並傳 options，不經 `RootCmd.Execute`。
