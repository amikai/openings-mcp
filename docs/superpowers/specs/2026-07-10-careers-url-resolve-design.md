# Careers URL 直接查詢設計

日期:2026-07-10
狀態:設計定案,待實作規劃

## 背景與目標

統一公司工具(`search_jobs_by_company` 等)目前只認 curated roster 內的公司:Registry 由各 adapter 的 `Roster()` 在啟動時建成,`company` 參數必須命中 roster 的 slug 或 display name。但四個 ATS 的實際覆蓋面是成千上萬家公司,roster 只是我們策展過的子集。

本設計讓 `company` 參數**額外接受 careers 頁 URL**:使用者(或 LLM 透過 web search)拿到一家公司的 careers URL 後,不需要任何預先註冊即可查詢。這把統一工具的覆蓋範圍從「roster 內的公司」擴大為「任何在支援 ATS 上的公司」。

設計靈感來自 case-study/OpenPostings 的 seeded-source 模式:careers URL 本身就足以唯一識別一家公司並導出全部連線 config。

目標:

- 四個 ATS adapter(Workday、Greenhouse、Lever、Ashby)都支援 URL 解析
- 伺服器維持全程無狀態:URL 解析在單次呼叫內完成,不寫入任何狀態;後續呼叫重複傳同一個 URL 即可
- MCP 工具介面零變動(不加新工具、不加新參數),只更新 schema 描述文字
- 為未來的 in-memory registry(URL 查過的公司可用名字再查)留下自然的擴充點,但本期不做

非目標:

- in-memory 或持久化的 runtime 公司註冊
- 104 / Cake / LinkedIn / Google / NVIDIA / TSMC 等非統一 ATS provider 的 URL 支援
- URL 深連結到單一職缺頁(`/job/...`)時順便解析出 job_id——一律解析到公司層級
- 從 upstream 撈 display name(Greenhouse/Ashby API 有回 board/org 名稱,但為此多打一次 API 不值得)

## 核心概念:slug 與 roster key 分離

現況中兩者是同一個字串:`Resolve` 回傳的 slug 永遠查得到 adapter 自家的 roster map(`ats.go` 註明「a slug that reaches an adapter is always one it declared」)。URL 解析出的公司沒有 roster entry,這個不變量因此放寬:

- **slug**:adapter 認得的公司識別字串,`Registry` 與 `Adapter` 之間的流通格式(定義不變)。
- **roster key**:slug 的其中一種形式——查得到 adapter config 表的那種。

Greenhouse/Lever/Ashby 的連線 config 只有一個值(board token / org),slug 的兩種形式天然重合,URL 解析出的 slug 跟 roster key 長得一樣,adapter 現有的 Search/Filters 路徑不需感知差異。Workday 的 config 是 tenant + instance + site 三個值,roster key(tenant 名)裝不下,因此 **roster 外的 Workday 公司以 canonical careers URL 當 slug**——URL 是這三個值的天然編碼,adapter 收到後重新 parse(純函式,零成本)導出 config。

## Adapter interface 變更

`ats.Adapter` 增加一個方法:

```go
// ParseCareersURL reports whether u is a careers-page URL on this ATS,
// and if so returns the slug that addresses that company. The slug may
// be a roster key or a self-describing form (workday returns the
// canonical careers URL for tenants outside its roster).
ParseCareersURL(u *url.URL) (slug string, ok bool)
```

收 `*url.URL` 而非 string:`Resolve` 解析一次,四個 adapter 共用,malformed URL 在進 adapter 前就被擋掉。

## 各 adapter 的 parse 規則

| Adapter | 認的 host | slug |
|---|---|---|
| Greenhouse | `job-boards.greenhouse.io`、`boards.greenhouse.io` 及其 `.eu` 變體(`job-boards.eu.greenhouse.io`、`boards.eu.greenhouse.io`) | path 第一段 = board token |
| Lever | `jobs.lever.co`、`jobs.eu.lever.co` | path 第一段 = org |
| Ashby | `jobs.ashbyhq.com` | path 第一段 = org(URL-decode) |
| Workday | `<tenant>.<instance>.myworkdayjobs.com` | 見下 |

Greenhouse/Lever/Ashby 共同規則:path 第一段之後的內容(職缺深連結、`/jobs` 尾綴等)一律忽略;path 為空(只有 host)視為 parse 失敗。

Workday 的 path 規則:

1. 跳過開頭的 locale 段:格式 `xx` 或 `xx-XX`(如 `en-US`、`zh-TW`),大小寫不拘。
2. 下一段是 site;再深的路徑(`/details/...`、`/job/...`)忽略。
3. host 拆出 tenant(第一段)與 instance(第二段);instance 需以 `wd` 開頭(涵蓋 `wd5`、`wd103`、`wd5-impl` 等形式),否則 parse 失敗。host 必須恰為四段(`tenant.instance.myworkdayjobs.com`),擋掉 `www.myworkdayjobs.com` 這類行銷頁。
4. tenant(小寫)查 `CompaniesByTenant`:命中回 roster key(如 `"nvidia"`),display name 與現有行為完全一致;miss 回 canonical URL:`https://<host>/<site>`(去 locale、去深路徑、去 query/fragment)。

決策補充(2026-07-10,PR #112 review 後):Workday 的另一個公開網域 `myworkdaysite.com` **刻意不支援**。它的真實 URL 把 tenant 放在 path(`wd<N>.myworkdaysite.com/<locale?>/recruiting/<tenant>/<site>`)而非 host,且經驗證(4/4,跨 wd1/wd3/wd5)每個發佈 myworkdaysite URL 的 tenant 都同樣可透過 myworkdayjobs.com 形式存取,故不值得為其維護第二套 path 解析。調查紀錄見 issue #113。

Greenhouse/Lever/Ashby 的 parse 不查 roster:slug 兩種形式重合,查了行為也無差。

## Resolve 流程

`Registry.Resolve` 的順序:

1. 現有路徑:normalize 後查 bySlug、byName。命中即回,不變。
2. miss 且輸入像 URL——有 scheme,或同時含 `.` 與 `/`;無 scheme 時補 `https://` 再 `url.Parse`——依 adapter 註冊順序輪詢 `ParseCareersURL`,first hit wins,回 `(adapter, slug)`。
3. 全 miss 時分流錯誤:
   - URL 形輸入:不跑 levenshtein(對 URL 算編輯距離沒有意義),回 teaching error「不是支援的 ATS careers 頁格式」並列出四種 host pattern。
   - 非 URL 輸入:現有 suggestion 錯誤,不變。

`Registry` 為此保存 `adapters []Adapter`(現在建完 maps 就丟棄 adapter 清單)。

URL 命中 roster 內公司時(如貼 NVIDIA 的 Workday URL),Workday 的 parse 規則第 4 步已把它折回 roster key,行為與用名字查詢完全一致。

## Adapter 內部改動

**Workday** — `client(slug)` 改為兩段式:

1. `CompaniesByTenant[slug]` 命中走現有路徑。
2. miss 時嘗試把 slug 當 careers URL 重新 parse(與 `ParseCareersURL` 共用同一個解析函式),成功則組出 CxS base URL(`https://<host>/wday/cxs/<tenant>/<site>`)。
3. 再 miss 才回錯誤。錯誤訊息從「internal inconsistency」改為 teaching error——slug 現在可能源自使用者輸入的 URL。

`Detail` 的 `Company` 欄位:ephemeral 公司用 tenant 名。現有 `baseURL func(Company) string` 測試縫只覆蓋 roster 路徑;URL 路徑的 mock 策略(如改為攔截 http.Client 層)在實作規劃時決定。

**Greenhouse/Lever/Ashby** — `Search`/`Filters` 零改動(slug 直接打 API,本來就不查表)。`Detail` 的公司名查詢(`CompaniesByBoardToken[slug].Name` 等)對 roster 外公司回空字串,fallback 用 slug 當公司名。涉及 `greenhouse.go`、`lever.go`、`ashby.go` 各一處。

## MCP 層

程式碼零改動,只更新文字:

- `companySearchInputRawSchema` 的 `company` 描述,與 `companyFiltersInput`、`companyDetailInput` 的 jsonschema tag:加上「或支援 ATS 的 careers 頁 URL,例如 `https://jobs.lever.co/acme`」。
- `search_jobs_by_company` 的 tool description:提一句「公司不在支援名單時,可直接傳該公司的 careers 頁 URL」。
- server instructions:補充教學——公司不在 roster 時,LLM 可先 web search 找出 careers URL 再查。

## 錯誤處理

- URL 格式對但 upstream 不存在(board 404、tenant 打不通):沿用各 adapter 現有的錯誤包裝(Greenhouse 已有「board not found upstream」),不加新機制。
- URL 形輸入 parse 全 miss:見 Resolve 流程第 3 步的 teaching error。

## 測試

- 各 adapter `ParseCareersURL` 的 table test:合法變體(locale 前綴、深路徑、`.eu` host、URL-encoded org、無 scheme)與非法輸入(錯誤 host、空 path、instance 格式不符)。
- `Registry.Resolve` 的 URL 分支:roster 命中折回、ephemeral 回傳、no-match teaching error、非 URL 輸入不受影響。
- Workday adapter 以 URL slug 走 `Search`/`Detail` 打 mock server 的整合測試。
- Greenhouse/Lever/Ashby 的 `Detail` 公司名 fallback。
