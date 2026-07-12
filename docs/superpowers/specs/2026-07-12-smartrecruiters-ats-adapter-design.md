# SmartRecruiters ATS adapter 設計

日期:2026-07-12
狀態:已實作(PR #143;本文件與實作同 PR 落地)
前置文件:[2026-07-06-unified-company-tools-design.md](2026-07-06-unified-company-tools-design.md)(Adapter interface 與家族概念出自該文件)

## 範圍

把 PR #141 建好的 SmartRecruiters Posting API client(`internal/provider/smartrecruiters`,含 52 家策展 roster)接進統一 company 工具。改動面:

- 新增 `internal/ats/smartrecruiters.go` 一個 adapter
- `cmd/openings-mcp/main.go` 註冊進 registry
- `registry.go` 的 careers URL 提示表加一行

不做的事:MCP 工具 schema、`internal/ats` 既有型別與引擎、provider client 皆零改動——這正是統一層當初的驗收條件(「加新 ATS 只需新增一個 adapter 檔案」)。

## MCP 層看到什麼

**沒有新工具、沒有新參數。** LLM 看到的仍是同三個工具:

```
search_jobs_by_company(company, query?, location?, filters?, page?)
get_filters_by_company(company)
get_job_detail_by_company(company, job_id)
```

差別只在解析範圍變大:

| 面向 | 變化 |
|---|---|
| roster | +52 家(`Equinox`、`BoschGroup`、`Ubisoft2`⋯),slug 與顯示名照常進 registry 雙索引,啟動時撞名檢查涵蓋(已驗證與既有四家 ATS 無撞名) |
| careers URL | `jobs.smartrecruiters.com/<company>` 成為第五種可直接貼 URL 的 host;roster 外的公司也能搜 |
| `get_filters_by_company` | SmartRecruiters 公司回單一維度 `{"department": [labels...]}` |
| 錯誤訊息 | 「unrecognized careers URL」的 host 清單多列一種 pattern |

兩個實作家族對 LLM 不可區分的不變式維持:同語意、同回傳形狀、同教學錯誤風格。

## 家族定位:第三種形態

既有兩家族:Workday(伺服器端搜尋,有 facet 探測 endpoint)、Lever/Ashby/Greenhouse(full-dump,一次抓整板進 `filter.go` 引擎)。SmartRecruiters 兩邊都不完全是:

- **有**真的伺服器端過濾:`q`(職稱關鍵字)、`city`/`region`/`country`、`department`(吃 id)、`limit`/`offset`(上限 100)
- **沒有** facet/維度枚舉 endpoint,也沒有一次拿整板的方式(list 端點單頁上限 100)

**full-dump 路線被板規模否決**:實測 roster 內 Bosch 4,720 筆、AECOM 4,870 筆——每次 Search 走整板 = 約 50 次上游請求,不可接受(Greenhouse/Ashby 是單一肥請求才付得起 dump)。所以 Search 走伺服器端,兩個統一參數在 API 沒有直接落點,各用一個機制翻譯:

| 統一參數 | 落點 | 機制 |
|---|---|---|
| `Query` | `q` | 直傳(上游語意:比對職稱) |
| `Location` | `city` / `region` / `country` 三選一 | **探測階梯**:依序發 `limit=1` 探測請求,誰的 `totalFound > 0` 就用誰;country 探測前先小寫(上游只認小寫 ISO code)。最多 3 次輕請求 + 1 次真搜尋,對應 Workday 的 facet 探測(無狀態的固定成本) |
| `Location = "remote"` | 無 | 教學錯誤。上游收下 `remote=true` 但**靜默忽略**(實測三家公司 totalFound 不變)——寧可明說做不到,不給一個看似生效實則全匹配的過濾 |
| `Filters` | `department`(唯一合法 key) | label → id:走整板收集 `department.{id,label}`(無枚舉 endpoint,這是唯一辦法);`get_filters_by_company` 共用同一次走訪 |
| `Page` | `offset` + `limit=20` | 直傳;`totalFound` 直接餵 `TotalCount`/`TotalPages` |

## 關鍵設計決策

### 1. department 只收單值

實測重複參數(`department=A&department=B`)上游只取一個、不做 OR;逗號串接也一樣。統一語意說「同 key 內 OR」,做不到就明講:兩值以上回教學錯誤(`takes exactly one value`),而不是靜默丟掉第二個值。曾考慮每值各發一次請求再合併——分頁與 totalCount 語意會壞掉,否決。

### 2. 整板走訪的成本形狀

`walkPostings`:第一頁拿 `totalFound` 後,其餘頁以 errgroup 併發(上限 8)抓,100 筆/頁。只有兩條路徑付這個錢——`get_filters_by_company` 與帶 `department` filter 的 Search;純關鍵字/地點搜尋永遠是 1(+探測)次請求。最壞情況(Bosch)約 50 次請求、一次呼叫內;典型 roster 公司 1–7 次。與 Ashby「detail 重抓整板」同一哲學:server↔上游的頻寬換無狀態,LLM 無感。

防禦:第一頁短於請求量(< 100)即視為整板結束,不信 `totalFound`——這同時讓 provider 的 mock(固定回 5 筆)自然可用,也擋上游數字不一致。

### 3. 摘要 URL 用 id-only 公開頁

list 回應沒有公開 URL 欄位(只有 API 自身的 `ref`;人類可點的 `postingUrl` 只在 detail 出現)。摘要改組 `https://jobs.smartrecruiters.com/<companyIdentifier>/<postingId>`——canonical URL 其實帶 SEO 標題後綴,但 id-only 形式實測直接 200 出同一頁。companyIdentifier 優先取回應內的 `company.identifier`(canonical 大小寫),fallback 到 slug。

### 4. empty-200 quirk 的兩個後果

上游對不存在的 companyIdentifier 回 **HTTP 200 + totalFound 0**,與「真公司但零職缺」同形,無法區分(只有 per-posting detail 端點才 404):

- careers URL 解析出的 roster 外公司,查無職缺時就是安靜的空結果,adapter 不編造錯誤(誠實優先)
- `ParseCareersURL` 特別拒收保留路徑 `sr-jobs`(跨公司搜尋頁 `jobs.smartrecruiters.com/sr-jobs/search`)——不擋的話會變成一個永遠回空板的假公司

### 5. slug 大小寫

roster 的 companyIdentifier 非統一小寫(`Equinox`、`AECOM2`),與 Greenhouse 的 board token 不同。API 端大小寫不敏感,registry 端 `normalize()` 也吃得下;`ParseCareersURL` 對 roster 內公司折回 roster 原始大小寫,讓顯示名解析與 URL 解析走到同一個 slug。Detail 的 `Company` 欄位優先序:roster 顯示名 → 上游 `company.name` → slug。

## 錯誤處理

| 情況 | 行為 |
|---|---|
| `location = "remote"` | 教學錯誤,發任何上游請求之前就擋 |
| location 三段探測全落空 | 教學錯誤:建議 city / 州代碼 / ISO country code,並註明也可能是公司本身無職缺 |
| filter key 非 `department` | 既有 `errUnknownFilterKey`,列出合法 key |
| department 值 ≠ 1 個 | 教學錯誤(見決策 1) |
| department label 查無 | 教學錯誤,列當下合法 label(最多 20 個,同 Workday 慣例) |
| `job_id` 查無 | 上游 404(`PostingErrorResponse`)→「pass a job_id exactly as returned by the job search」,與各家族同款 |
| 未知公司 | 空結果,無錯誤(quirk,見決策 4) |

## 測試策略

provider 的 mocksrv 不懂 query 參數與分頁,adapter 測試自建一個模擬 Posting API 語意的 fake server:250 筆假板(跨 3 個走訪頁)、offset/limit 分頁、`q`/`city`/`region`/`country`(只認小寫)/`department` 過濾、unknown company 的 empty-200。重點案例:

- 探測階梯三層各命中一次(`Houston`→city、`TX`→region、`US`→小寫後 country)、全落空報錯
- 「只在最後一頁出現的 department」證明走訪完整;請求計數器(atomic,走訪是併發的)釘住成本形狀:無過濾搜尋 = 每頁 1 次、250 筆板走訪 = 3 次、remote = 0 次
- detail 的 jobAd sections → 純文字(含無標題 section 的 fallback 標題)、roster 顯示名優先序
- `ParseCareersURL`:roster 大小寫折回、`sr-jobs` 拒收

上游行為(2026-07-12 對 Equinox/ServiceNow/Wix/Experian 實測):伺服器端過濾真的收斂 totalFound、country 只認小寫、`remote=true` 被忽略、重複參數不 OR、id-only 公開 URL 200——這些都寫進 adapter 註解,測試的 fake server 照此模擬。

## 後續(非本期)

### 1. custom_field 過濾維度

每家公司的 posting 都帶一串 tenant 自定義的 `customField`(2026-07-13 抽 8 家實測:9–17 個維度/家),其中不少對求職者有真實價值——雇用型態(Bosch `Contract type`)、工作模式(ServiceNow `Work Persona: Remote`,可部分補上本期 remote 教學錯誤的缺口)、職能(Ubisoft `Job Family`)、甚至薪資範圍(Experian)。上游的 `custom_field.{fieldId}=<valueId>` 是 openapi.yaml 已標 verified 的伺服器端過濾,而 `Filters()` 的整板走訪本來就抓回了 customField 陣列——**枚舉的邊際成本是零**,要做的是 `Filters()` 擴維度 + Search 端 label→(fieldId, valueId) 解析。待解:雜訊維度過濾(Cost Center、BILL CODE 之類列進 get_filters 只是浪費 context)、fieldLabel 撞名(與 `department` key、與彼此)、逐 field 抽驗可過濾性。MCP schema 零改動。

### 2. sr-jobs 跨公司 feed —— 評估過,本期決定不做成工具

`jobs.smartrecruiters.com/sr-jobs/search` 是官方搜尋頁的現役後端(同 URL 依 Accept 協商回 JSON),undocumented 但形狀與 Posting API list 幾乎同款,支援 `keyword=`、`company=`、`offset`/`limit`。2026-07-13 實測兩個關鍵行為:

- **它是 Posting API 的子集,差在時效**:同查詢(Equinox + trainer)feed 122 筆 vs API 138 筆,feed 缺的 42 筆結構欄位全同(PUBLIC/en),差在多半是 2025 以前的舊 evergreen 缺——feed 是有 freshness 篩選的搜尋索引,Posting API 才是 source of truth
- feed 每筆的 department/industry/customField 皆空,只給搜尋頁顯示用欄位

做成 `smartrecruiters_search_jobs`(job board 家族,同 104/Cake/LinkedIn)技術上可行,detail 可跳回官方 API。**決定不做**:「搜整個 SmartRecruiters 平台」不是求職者的真實意圖切面(ATS 對求職者是看不見的水管——統一層的設計原則),平台無目標探索已有 LinkedIn 等 job board 覆蓋;多一個工具吃 context 又讓 LLM 路由變難;undocumented endpoint 的 quirk(freshness、與官方 API 對不上)得負責到底。feed 的真實價值目前在 offline:roster 策展就是從它挖的。

重啟條件:出現反覆的「以平台為範圍」探索需求;或想做 Resolve 未命中時的動態公司發現(feed 的 `company=` 可拿來查 companyIdentifier)——後者是擴 registry,不是加工具。
