# 統一 company 工具實作設計

日期:2026-07-06
狀態:設計定案,待實作規劃
前置文件:[2026-07-05-ats-unified-search-design.md](2026-07-05-ats-unified-search-design.md)(高層設計;本文件是它的落地層)

## 範圍

把既有的 workday、lever、ashby 三個 provider 統一到一組以 company 為參數的 MCP 工具後面。三個工具一次到位:

```
search_jobs_by_company(company, query?, location?, filters?, page?)
get_filters_by_company(company)
get_job_detail_by_company(company, job_id)
```

**本期不含 greenhouse**:它的 provider client 已存在,但 roster(companies.yaml)尚未策展。本設計保證 greenhouse 之後加入只需在 `internal/ats` 新增一個 adapter 檔案 + roster,其他層零改動。

其餘不做的事沿用前置文件:不做跨公司 fan-out、不做 `list_companies`、不動既有 per-company 工具(nvidia 等保留)、ORC/SmartRecruiters 等新 ATS 不在本期。

## 架構

```
internal/provider/*        既有,零改動 —— 純 API client(ogen 生成)+ curated roster
        ▲
internal/ats/              新 package —— 統一語意層
  ats.go                     Adapter interface + 統一型別
  registry.go                Registry + Resolver(union 各 provider roster)
  filter.go                  full-dump 家族共用的過濾/排序/分頁引擎
  workday.go                 Workday adapter(伺服器端搜尋家族)
  lever.go                   Lever adapter(full-dump 家族)
  ashby.go                   Ashby adapter(full-dump 家族)
        ▲
internal/openingsmcp/
  company.go               三個 *_by_company MCP 工具
        ▲
cmd/openings-mcp/main.go   組裝:http.Client → 三個 adapter → registry → 註冊工具
```

分層職責:provider = 純 client,ats = 統一語意(參數轉譯、過濾、排序、名稱解析),openingsmcp = MCP schema 轉譯。曾考慮的替代方案——邏輯直接寫進 openingsmcp(職責混雜、CLI 無法重用)、adapter 分散到各 provider package(把統一語意塞進純 client 層,過濾邏輯得跨 package 共用)——皆否決。

關鍵不變式:

- **無狀態**:每次 tool call 重新走完整流程,無任何快取。workday adapter 按公司臨時建 `workday.Client` 是零成本包裝,連線池在共用的 `http.Client` 上
- **兩個實作家族對 LLM 不可區分**:同樣的參數語意、同樣的回傳形狀
- **JD 全文不進搜尋結果**(context 紅線),只有 `get_job_detail_by_company` 回全文

## 統一型別與 Adapter 介面

```go
// Adapter 是一家 ATS 的統一搜尋實作。方法以 slug 定位公司,
// slug 由 Roster() 對外宣告、registry 統一索引。
type Adapter interface {
    Name() string                // "workday" / "lever" / "ashby",僅供日誌與錯誤訊息
    Roster() []CompanyInfo       // 這家 ATS 上所有已策展公司
    Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error)
    Filters(ctx context.Context, slug string) (FilterSet, error)
    Detail(ctx context.Context, slug, jobID string) (*JobDetail, error)
}

type CompanyInfo struct {
    Slug string // 唯一鍵,即 provider roster 的 tenant/site/board slug
    Name string // 顯示名,resolver 也拿來做名稱比對
}

type SearchParams struct {
    Query    string              // 關鍵字:職稱、技能、技術
    Location string              // 模糊文字比對;"remote" 特判
    Filters  map[string][]string // 逃生口,值來自 get_filters;同 key OR、跨 key AND
    Page     int                 // 1-based,固定每頁 20 筆
}

type SearchResult struct {
    Jobs       []JobSummary
    TotalCount int
    Page       int
    TotalPages int
}

type JobSummary struct {
    JobID    string // provider 原生 id(workday=externalPath、lever=uuid、ashby=id)
    Title    string
    Location string
    PostedAt string // ISO 8601 日期,盡力而為
    URL      string // 人類可點的職缺頁
}

type FilterSet map[string][]string // 維度 → 當下合法值(顯示用 label)

type JobDetail struct {
    JobID       string
    Title       string
    Company     string
    Location    string
    PostedAt    string
    URL         string
    Description string // 一律正規化為純文字
}
```

設計理由:

- **Adapter 方法吃 `slug string` 而非 provider 專屬型別**——避免 `any` 與 type assertion。`CompanyInfo` 只服務 registry 的名稱解析,不承載連線設定;workday 需要的 tenant/instance/site 留在 `workday.CompaniesByTenant`(直接用套件變數,不包 getter),adapter 內部以 slug 換回完整 config。lever/ashby 的 slug 本身即全部所需參數。adapter 內 slug 查不到 = registry 與 roster 不同步的 internal bug(理論上不可能,`Roster()` 出自同一份資料),回一般錯誤
- **`Roster()` 是 registry 的唯一資料來源**——registry 不認識任何 provider package,加新 ATS 不改 registry
- **`FilterSet` 值是 label 不是 id**——對 LLM 友善;Workday 的 label→GUID 轉換由 adapter 在 Search 時解決
- Description 來源:workday JD 是 HTML,過既有依賴 `html2text`(nvidia.go 已在用);lever JD 分段(見 lever adapter 一節);ashby 用 `descriptionPlain`

## Registry 與 Resolver

```go
type Registry struct { /* 唯讀索引 */ }

func NewRegistry(adapters ...Adapter) (*Registry, error)
func (r *Registry) Resolve(company string) (Adapter, string, error)
```

**建構**:逐一收各 adapter 的 `Roster()`,建兩個索引——slug → entry、正規化顯示名 → entry。正規化 = 小寫、去頭尾空白、去標點與內部空白(`"Workday, Inc."` → `"workdayinc"`)。**slug 或正規化顯示名跨 adapter 撞名 → `NewRegistry` 回 error,server 拒絕啟動**——roster 是策展資料,撞名是資料 bug,要在啟動時炸出來而不是靜默蓋掉(目前 196 + 16 + 46 家無撞名)。

**解析——樂觀呼叫 + 教學錯誤**:

1. 輸入正規化後,先查 slug 索引,再查顯示名索引 → 命中即回,零額外成本
2. 未命中 → 模糊比對產生建議:substring 命中優先,否則 edit distance 取最近 3 筆
3. 錯誤訊息即探索介面:
   > `unknown company "adobe systems"; closest matches: adobe, autodesk, asana. 258 companies are supported — pass one of the suggested slugs.`

alias(「輝達」→ nvidia)本期不做:roster 無 alias 資料,slug + 顯示名雙索引加模糊建議已涵蓋主要情況;之後有需求在 ats package 加一層 alias 表,不動介面。

## 三個 Adapter 的行為

### Workday(伺服器端搜尋家族)

| 統一參數 | 落點 |
|---|---|
| `Query` | `searchText` |
| `Location` | locations facet:label 模糊比對 → GUID 進 `appliedFacets` |
| `Filters` | facet label → GUID → `appliedFacets`(同 key OR、跨 key AND,Workday 原生語意) |
| `Page` | `offset` + `limit=20` |

- **Label→GUID 二段請求**:`appliedFacets` 吃 GUID,但 `get_filters` 回 label。無狀態下 adapter 在 Search 時先打一次 `limit=1` 探測請求拿當下 facet 樹,把 label 解析成 GUID,再發真正的搜尋——帶 `Location`/`Filters` 的 workday 搜尋 = 2 次上游請求,不帶則 1 次。無狀態的固定成本,接受
- Label 解析不到 → 教學錯誤,列出該維度當下合法值
- `Filters()` = 一次 `limit=1` 搜尋,把回應的 facet 樹(含巢狀 group)攤平成 `FilterSet`
- `Detail()` = 原生 `GetJobDetail`,JD HTML 過 `html2text`
- Client 建構:per-call 以 `workday.Company.BaseURL()` 臨時建 `workday.Client`,共用注入的 `http.Client`
- Roster 特例:fox / dowjones 各有兩列股權類別共用同一 tenant(已驗證 companies.yaml 409/413、690/694 行),兩列解析到同一 BaseURL。**workday adapter 的 `Roster()` 以 slug 去重(保留第一列)**,避免 registry 收到重複 slug;跨 adapter 的撞名檢查不受影響

### Lever / Ashby(full-dump 家族,共用 `filter.go`)

Search 流程:抓整包(`ListPostings` / `GetJobBoard`)→ 每筆轉成 `filter.go` 的中間形狀(摘要欄位 + 可搜尋文字欄位 + 結構化欄位)→ 統一引擎過濾、排序、切頁。

`filter.go` 引擎(純函式,無 I/O):

- `Query`:大小寫不敏感、多詞 AND;比對優先序 title > team/department > JD 內文(兩家的 `descriptionPlain`)
- `Location`:子字串比對 location 欄位;`"remote"` 特判走 `isRemote`/`workplaceType`(兩家都有明確欄位)
- `Filters`:結構化欄位精確比對——lever: `team`/`commitment`/`workplaceType`,ashby: `department`/`team`/`employmentType`/`workplaceType`
- 排序:title 命中優先 → `posted_at` 新→舊 → id 決勝(決定性,無狀態分頁靠這個)
- 分頁:offset 切片,每頁 20

其他:

- `Filters()` = 抓整包後對結構化欄位做 distinct 枚舉
- Detail:lever 走原生單筆 `GetPosting`;**ashby 無公開單筆 endpoint(實測 401)→ 重抓整包挑 id**,一次肥請求(數 MB)發生在 server↔Ashby 之間,LLM 無感
- Ashby 只列 `isListed=true` 的職缺
- Lever JD 組成(欄位分段,plain 版本齊全):`descriptionPlain`(opening + 描述)+ `lists`(各區塊 `text` 標題 + `content` HTML 過 `html2text`)+ `additionalPlain`(結語),依序串接為全文

## MCP 層

`internal/openingsmcp/company.go`,照 linkedin.go 的既有模式:手寫 JSON schema + 輸入/輸出 struct + Register 函式。

```go
func RegisterCompany(s *mcp.Server, r *ats.Registry)
```

| 工具 | 輸入 | 輸出 |
|---|---|---|
| `search_jobs_by_company` | `company`(必填)、`query`、`location`、`filters`(object,值為 string array)、`page` | `data`(統一摘要列表)+ `total_count`、`page`、`total_pages`;schema 預留 `next_cursor`(不實作) |
| `get_filters_by_company` | `company`(必填) | 維度 → 合法值列表 |
| `get_job_detail_by_company` | `company`(必填)、`job_id`(必填) | 統一 `JobDetail`(純文字 JD) |

- 工具 description 寫明:company 吃公司名或 slug、認不得時錯誤會給建議、filters 的值要來自 `get_filters_by_company`
- `main.go`:建三個 adapter(共用 `http.Client{Timeout: 30s}`)→ `ats.NewRegistry(...)` → `RegisterCompany(server, registry)`
- `serverInstructions` 補統一工具的路由指引:指名公司且不在 per-company 工具清單時優先用 `*_by_company` 工具
- 既有 per-company 工具保留不動

## 錯誤處理總表

| 情況 | 行為 |
|---|---|
| company 解析失敗 | 教學錯誤(建議 + 支援家數),`errorResult` |
| filter label / location label 解析失敗(workday) | 教學錯誤,列出該維度當下合法值 |
| filters 的 key 不存在(full-dump 家族) | 教學錯誤,列出合法 key |
| 上游 HTTP 錯誤 / timeout | `errorResult`,訊息帶 adapter 名與公司,供 LLM 轉述 |
| `job_id` 查無 | 上游 404 原樣轉譯;ashby(整包挑 id)由 adapter 自己回 not found |
| slug / 顯示名撞名(啟動時) | `NewRegistry` 回 error,server 拒絕啟動 |

所有 tool call 層級的錯誤走既有 `errorResult`(IsError,不炸 protocol)。

## 測試策略

| 對象 | 方式 |
|---|---|
| `filter.go` 引擎 | 純函式單元測試:query AND/優先序、location + remote 特判、排序決定性、分頁切片 |
| Registry / Resolver | 單元測試:正規化、slug/顯示名雙索引、模糊建議、撞名報錯 |
| 三個 adapter | 對各 provider 既有的 `mocksrv.go` 打真請求:參數轉譯(workday 的 label→GUID 二段請求)、欄位正規化、ashby detail 整包挑 id |
| MCP 層 | 照既有 provider 工具的測試慣例,in-memory transport 走一輪三工具 |

## 後續(非本期)

1. Greenhouse:roster 策展(掃描 + 驗證腳本)後,加 `internal/ats/greenhouse.go` 即接入
2. alias 表(中文名、俗名 → slug)
3. `next_cursor` keyset 分頁升級(schema 已預留)
