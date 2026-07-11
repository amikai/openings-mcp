# Provider schema nullable 掃描 — 設計

日期：2026-07-11
狀態：已與使用者確認設計，待實作

## 背景與目標

透過真正的 MCP server 對 Greenhouse 抽測 30 家公司，17 家 search 直接失敗、另 4 家 detail 失敗（issue #123）。根因是 `internal/provider/greenhouse/openapi.yaml` 把 `Office.location`、`JobDetailMetadataItem.value` 宣告成單純 `string`，但上游真實資料會送 `null`（`value` 甚至會送陣列），ogen 生成的解碼器遇到就整包報錯——一個欄位壞掉，該公司的搜尋結果全部拿不到。

這類 provider 全部是反向工程來的非官方 API，schema 是用觀察猜的，不是官方保證。Workday/Lever/Ashby 這次抽測的 30 家都過了，但那只代表「今天抽到的樣本沒踩雷」，不代表 schema 本身夠寬鬆。既然放寬 nullable 對「上游其實從不送 null」的欄位是零風險（解碼器只是多接受一種值），這次就把這條原則套到全部 provider spec，而不是只補 Greenhouse 這两個欄位。

## 已確認的決策

1. **範圍**：`Makefile` `OPENAPI_SPECS` 列的 10 個 spec（cake, google, greenhouse, job104, lever, linkedin, nvidia, synopsys, tsmc, workday）+ Ashby 的 openapi.yaml（目前沒被列進 `validate-openapi`，順手補上）。
2. **分類規則**：
   - **Identifier 欄位**（job ID、slug、tenant、board_token 等會被拿去當下一次 API 呼叫的 key）→ 維持嚴格 required/typed，不放寬。這裡出現 null 代表資料真的壞了，應該讓整包解碼失敗、报错出来，不要吞掉變成空字串到處流竄。
   - **其他有被 adapter/tool 程式碼讀取的欄位**（展示、篩選、排序用，如 location、title、日期、部門名稱）→ 標成 `nullable: true`。
   - **完全沒被消費、且真實資料已顯示型別不穩定的欄位**（如 Greenhouse 的 `metadata[].value`：實測看過 null 與陣列兩種）→ 拿掉 `type:` 限制，讓 ogen 生出 untyped/raw 型別，不強行解析。
3. **驗收方法**：不靠「先抓全部上游原始 payload 比對」這種證據驅動的方式決定要改哪裡（成本高，而且今天沒 null 的欄位明天可能就有）。改用**規則驅動 + 全 roster 實測驗收**：規則掃 spec 決定怎麼改，改完後把四家 ATS 的完整 curated roster（Workday 201 + Lever 20 + Ashby 71 + Greenhouse 68 = 360 家）透過真正跑起来的 MCP server 全部跑一輪 `search_jobs_by_company`；對有回結果的公司，額外對第一筆結果跑 `get_job_detail_by_company`。全過才算收工。
4. **Non-goal**：不改任何 adapter 的商業邏輯，不加 runtime 層容錯（例如「單筆解碼失敗就跳過」）。目的是讓 schema 準到不需要這種容錯；如果 360 家驗收後仍有真的過不了的，那是新資訊，回來再議是否要加，不在這次方案裡先做。

## 設計

### 為什麼是零程式碼改動（schema-only）

ogen 對 `type: string` 加上 `nullable: true` 後，生成型別從 `OptString` 換成 `OptNilString`：

```go
type OptNilString struct {
    Value string
    Set   bool
    Null  bool // 新增
}
```

`.Value` 欄位保留，遇到 `null` 時 `.Value` 就是空字串（zero value）——跟現在「欄位不存在」時的行為完全一致。Greenhouse 生成碼裡已有 177 處這樣宣告的欄位（如 `RequisitionID`, `AiDisclaimer`），既有 adapter 程式碼讀 `.Value` 的地方不用改一行。同理，完全沒被消費的欄位放寬成 untyped 也不會動到任何呼叫端。

也因此，這次的實作範圍**只有 `.yaml` 編輯 + `go generate` + 編譯確認**，不涉及 `internal/ats/*.go`、`internal/openingsmcp/*.go` 或任何手寫邏輯的改動。若掃描過程中發現某個欄位其實有被消費、且放寬型別後呼叫端的假設會壞掉（例如某處把 `OptString` 直接當非 optional 使用），才需要額外修那一小段呼叫端程式碼——這種情況在計畫執行時個案處理，不預先假設會發生。

### 逐 provider 執行步驟

對 11 個 spec（10 個 Makefile 列的 + ashby）各自：

1. 讀對應的 adapter/tool 程式碼（統一 ATS 的四家看 `internal/ats/{workday,lever,ashby,greenhouse}.go`；其餘六家看 `internal/openingsmcp/{provider}.go` 與 `internal/provider/{provider}/companies.go`），列出實際被讀取的 schema 欄位。
2. 依上述規則逐欄位分類、編輯 `openapi.yaml`。
3. 執行該套件的 `go:generate` 指令（每個 provider 目錄的 `gen.go` 都有 `//go:generate go tool .../ogen ...`）重新產生 `oas_*.go`。
4. `go build ./...`，跑該 provider 既有的 `*_test.go`，確認沒有編譯或測試回歸。

### 補 Ashby 進 `validate-openapi`

`Makefile` 的 `OPENAPI_SPECS` 清單少了 `internal/provider/ashby/openapi.yaml`，這次順手加進去，讓它跟其他 10 個一樣被 CI 的 `validate-openapi` 驗證。

### 驗收：全 roster 實測

沿用這次抓 bug 用的做法：一支暫時性的 Go harness，透過 `mcp.CommandTransport` 啟動真正的 `openings-mcp` binary、用 `mcp.NewClient` 建連線，對 360 家 curated 公司（四個 registry 化的 ATS 全部，不再只抽樣）依序呼叫 `search_jobs_by_company`；`total_count > 0` 的公司再對第一筆結果呼叫 `get_job_detail_by_company`。輸出每家公司的 pass/fail 與錯誤訊息全文，彙總每個 provider 的通過率。

這支 harness 不進 repo（跟這次 issue #123 抓 bug 用的暫時檔案一樣，用完即刪），跑完後把最終結果貼進 PR 描述或留言在 issue #123，全過就直接關掉 issue；沒過的話附上失敗清單與錯誤訊息，作為是否要追加規則的依據。

## 測試

- 每個 provider：`go build ./...` + 該 provider 既有的 unit test（mock server 測資不含 null，不會抓到這類問題，純粹確保沒有回歸）。
- 全 repo：`go build ./...`、`go vet ./...`。
- 360 家 curated 公司的全量實測（見上）作為此次修復的最終驗收，取代 issue #123 原本 30 家抽樣的證據。
