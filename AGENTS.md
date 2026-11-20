# ocr_agent — 技術脈絡 (Technical Context)

本檔記錄結構、分層與關鍵決策。功能、task type 與 CLI 用法由 [README.md](README.md) 單一擁有。

## 目錄結構 (Directory Layout)

```tree
ocr_agent/
├── main.go                 # config.Default + 內嵌 config/example/settings.example.json -> cmd.ExecuteContext
├── config/example/settings.example.json  # `ocr_agent config default` 的 seed: 1 個 classify + 3 個 date task 與其 description (全 minimax / MiniMax-M3)
├── cmd/                    # cobra: 旗標、tasks 設定解析、輸出; 不做歸檔決策
│   ├── root.go             # RootCmd, 掛 gosdk ConfigCmd、各 type subcommand 與 run
│   ├── typecmd.go          # newTypeCmd: 一個 type 一個 subcommand, 以 --source 選一個 task
│   ├── classify.go         # ClassifyCmd
│   ├── date.go             # DateCmd
│   ├── run.go              # RunCmd: 遍歷 tasks[], 依設定順序
│   └── internal/           # cmd 共用的設定、執行迴圈與檔案列舉
│       ├── task.go         # Task (直接 decode tasks[])、驗證、FindTask (type + --source)、provider 建構
│       ├── run.go          # RunTask: 一個 task 逐圖執行 (進度 stdout, 失敗 stderr); RunEach: run 用的遍歷
│       └── files.go        # source 掃描與 --file 驗證
├── svc/                    # 領域邏輯, 不 import cobra / viper
│   ├── imagefile/          # 影像載入 (HEIC -> JPEG via sips) 與列舉
│   ├── task/               # Contract (type 規則)、Prompt 組成、mv agentSDK tool、Pipeline (每圖一次 model call)
│   ├── date/               # date 欄位驗證 (含民國年)
│   ├── target/             # targets 清單驗證、{yyyy}/{mm} 解析、Root
│   └── filing/             # 不覆寫的目的地挑選、Move、image + sidecar 原子提交
├── scripts/run.sh          # run:setup: 建 ~/.config/ocr_agent/{data,logs}、tmp/config symlink、補齊 seed key
└── docs/                   # terminology、memory、specs
```

相依方向：`cmd -> svc/task -> svc/{target,date,filing,imagefile}`，`svc/target -> svc/filing`。
`svc/` 只拿到已解析好的值 (provider、路徑)，對 viper 一無所知。

## 關鍵決策 (Key Decisions)

### 設定是 tasks 清單

`tasks[]` 每個 object 是一個完整 task：`name`、`description`、`type`、`provider`、`model`、`timeout`、`source`、`targets`（另有選填 `base_url`）。
`viper.UnmarshalKey` 直接 decode 進 `cmd/internal.Task` 與 `target.Target`（mapstructure tag），`resolve` 就地驗證並展開路徑；沒有平行的 entry struct。
這取代了舊版的 `domain.<name>` map、寫死在 code 的 prompt 與 `domain.<name>.target`。

`type` 對應 `svc/task.Contract`，是唯一寫在 code 的部分：

- `classify`：`Recursive`；`metadata` 自由、只記 log；搬原檔。
- `date`：`Dated`；`metadata.date` 必須有效（`svc/date.Parse`，民國年 +1911、不可未來）；搬原檔 + `metadata` sidecar；`{yyyy}/{mm}` 由 date 填入。

一個 type 一個 subcommand（`newTypeCmd`），同一 type 可有多個 task（seed：date 有 general / medical / special）。
subcommand 不遍歷清單：`FindTask(type, --source)` 取出 `source` 相符的唯一 task 直接交給 `RunTask`；該 type 只有一個 task 時 `--source` 可省略。
因此 `LoadTasks` 要求同 type 內 `source` 唯一。`run` 才遍歷 `tasks[]`（`RunEach` → 每個 `RunTask`），沒有依 type 分派的 switch。
model / provider 沒有 CLI 覆寫：task 設定是唯一來源。

### 每圖一次 model call，mv 是 agentSDK tool

`svc/task.Pipeline` 在建構時組好 prompt（`Prompt`：description + target 清單 + contract 的 metadata 格式）與 `mv` 的 `ToolSpec`；
每張影像送一次 `ModelRequest{prompt, 影像, Tools: [mv]}`。

- `mv` 是 `core.Tool`：schema 由 `tool.SchemaFor[MvArgs]` 反射，再把 `target` 收斂成 target name 的 enum；`Call` 走 `tool.CallWithRawMessage`（required 驗證、decode）；派送用 `tool.Registry.Call`。
- `MvArgs` 只有 `target` 與 `metadata`；要搬的檔永遠是當下這張影像，模型無法指定 source 或路徑。
- 沒有 `mv` 呼叫 = 留在原處；未知 target、缺 date、多次呼叫 = 該檔失敗。dry-run 照常呼叫模型並解析目的地，只略過搬移。
- agentSDK 的 builtin `mv`（任意路徑）不用：它不處理 `_2` 同名後綴與 sidecar 的原子提交；名稱同為 `mv` 但不會進同一個 registry。

### Targets 與 placeholder

- `{yyyy}` / `{mm}` 取代舊的固定 `<target>/<YYYY>/<MM>/`；`Target.Root()` 是 placeholder 之前的固定前綴，`EnsureWithin` 以它為界。
- 遞迴掃描的 task（classify）拒絕 `Root()` 位於自己 source 之內的 target；`Resolve` 拒絕檔案目前所在的 folder（同一 task 下次會再處理）。

### Provider 由 registry 名稱決定

`cmd/internal/task.go` blank-import `agentsdk/provider/all`，以 `provider.New(name, …)` 建構 adapter。換 vendor 是改設定，不是改 code。
`LookupEnv` 是 `viper.GetString`：gosdk 已把 OS env 與 `~/.config/ocr_agent/.env` 綁進 viper，所以 `MINIMAX_API_KEY` 兩處都有效。

### 沒有 code 內建預設值

provider / model / 路徑 / description 只來自設定 (seed 由 `config default` 寫入)，缺值時錯誤訊息指出 `tasks[i] "name"` 與欄位。
唯一的 code 預設是 `timeout` 的 `10m`。

## 測試 (Testing)

- `cmd/cmd_test.go` 在 agentsdk registry 註冊一個 `stub` provider（依 model 回固定的 `mv` 參數），以 `viper.Set` 建 temp 目錄的一個 classify 與兩個 date task；
  驗證 classify / date 搬到所選 target、失敗只寫 stderr、`--source` 只跑一個 task（唯一 task 可省略、多個時必填、不符時報錯）、`run` 依設定順序、target 位於遞迴 source 內被拒、與無效 task（含同 type 重複 source）錯誤。
- `svc/task` 以 stub model 驗證一次 request 帶 prompt（含 description、target 清單、格式）+ 影像 + enum 化的 `mv`、sidecar 內容、classify 不寫 sidecar、各種不搬移的情況。
- `svc/*` 測試不需要模型或真實影像；HEIC 測試會呼叫 macOS `sips`。
- Verification 不得讀取、辨識或搬移真實收據。

## 陷阱 (Pitfalls)

- `go mod tidy` 在 `main.go` 尚未 import gosdk 時會把它移除，之後再加回會解析到舊版 (曾降到 v1.3.5)；改完 import 後確認 `go.mod` 的版本。
- package-level flag 變數在同一 process 內跨 `Execute()` 保留值；測試直接呼叫 `FindTask` + `RunTask`（或 `LoadTasks` + `RunEach`）並傳 `RunOptions`，不經 `RootCmd.Execute`。
- `cmd/internal.Task` 是 viper decode 目標，`Contract` 以 `mapstructure:"-"` 排除；新增欄位要同時加 mapstructure tag，否則靜默讀不到。
