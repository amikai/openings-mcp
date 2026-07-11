package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCakeMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := cake.NewMockServer()
	t.Cleanup(srv.Close)
	client, err := cake.NewClient(srv.URL, cake.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterCake(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverSession.Close()
	})

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientSession.Close()
	})
	return clientSession, serverSession
}

func TestRegisterCake(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := cake.NewClient("https://api.cake.me")
	require.NoError(t, err)
	RegisterCake(server, client)

	assertTools(t, server, "cake_search_jobs", "cake_get_job_detail")
}

func TestCakeSearchJobsE2E(t *testing.T) {
	clientSession, _ := testCakeMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "cake_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	// Full golden schema: LLM-facing names only (no query/sort_by/filters
	// nesting), keyword and location required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text keyword search.",
			},
			"location": map[string]any{
				"type":        "string",
				"description": "Location name as shown on Cake.me, localized English or Chinese, e.g. \"Taiwan\", \"台灣\", \"Taipei City, Taiwan\".",
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Employment type.",
				"enum":        []any{"full_time", "part_time", "internship", "contract", "freelance", "temporary", "volunteer"},
			},
			"seniority": map[string]any{
				"type":        "array",
				"description": "Seniority levels, OR'd together.",
				"uniqueItems": true,
				"items": map[string]any{
					"type": "string",
					"enum": []any{"internship_level", "entry_level", "associate", "mid_senior_level", "director", "executive"},
				},
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote-work policy. Omit to include all.",
				"enum":        []any{"no_remote_work", "partial_remote_work", "optional_remote_work", "full_remote_work"},
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Result order. Defaults to popularity.",
				"enum":        []any{"popularity", "latest"},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "location"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "cake_search_jobs",
		Arguments: map[string]any{"keyword": "Golang", "location": "台灣"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got cakeSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	wantResp := &cakeSearchOutput{
		TotalEntries: 360,
		TotalPages:   18,
		PerPage:      20,
		CurrentPage:  1,
		Data: []cakeJobSummary{
			{
				Path:  "senior-golang-web-backend-engineer-taoyuan",
				URL:   "https://www.cake.me/companies/lctech_/jobs/senior-golang-web-backend-engineer-taoyuan",
				Title: "資深 Golang 網頁後端工程師(桃園)",
				Description: `【職務內容】
維護現有系統與功能、新專案開發、跨部門團隊需求
標配MAC電腦
(本職務辦公室位於桃園，遠端工作需求面議)
【上班時間】
早上 9 點至下午 6 點並且有 30 分鐘彈性上下班

【公司福利】
勞健保、三節、尾牙、生日禮金。
我們公司設有咖啡休息區，免費零食、飲料、餐點，飲品，備有微波爐。
不定時員工聚餐與每年都有員工旅遊。`,
			},
			{
				Path:        "senior-backend-engineer-golang-123",
				URL:         "https://www.cake.me/companies/wit-software-ltd/jobs/senior-backend-engineer-golang-123",
				Title:       "Senior Backend Golang Developer / 資深後端 Golang 工程師",
				Description: `▎What You'll Do身為資深 Golang 後端工程師，你將參與支援百萬級用戶、高 TPS 的大型系統設計與開發，聚焦於即時通訊、社群互動、支付與平台功能模組，並攜手跨部門夥伴共同交付高品質產品。你的實際工作將包含：1. 建構與優化高併發、高穩定性、高可維護性的後端架構。2. 負責即時訊息、推播、社群互動、錢包支付等模組開發與持續演進。3. 深入參與系統資源調度與效能優化策略設計。4. 對問題進行深入剖析，提出具邏輯性與可落地的解法。5. 與產品、前端、UI/UX、測試團隊密切協作，從需求釐清到上線監控全流程參與。6. 評估技術選型，理解架構決策背後的 trade-off，打造可持續演進的系統。`,
			},
			{
				Path:  "4672975006-solution-engineer-c-golang-python-9f5e80aaa20eac33b0662a03591d00",
				URL:   "https://www.cake.me/companies/WorldQuant/jobs/4672975006-solution-engineer-c-golang-python-9f5e80aaa20eac33b0662a03591d00",
				Title: "Solution Engineer (C++ / Golang / Python)",
				Description: `WorldQuant develops and deploys systematic financial strategies across a broad range of asset classes and global markets. We seek to produce high-quality predictive signals (alphas) through our proprietary research platform to employ financial strategies focused on market inefficiencies. Our teams work collaboratively to drive the production of alphas and financial strategies – the foundation of a balanced, global investment platform.
WorldQuant is built on a culture that pairs academic sensibility with accountability for results. Employees are encouraged to think openly about problems, balancing intellectualism and practicality. Excellent ideas come from anyone, anywhere. Employees are encouraged to challenge conventional thinking and possess an attitude of continuous improvement.
Our goal is to hire the best and the brightest. We value intellectual horsepower first and foremost, and people who demonstrate an outstanding talent. There is no roadmap to future success, so we need people who can help us build it.Technologists at WorldQuant research, design, code, test and deploy firmwide platforms and tooling while working collaboratively with researchers and portfolio managers. Our environment is relaxed yet intellectually driven. We seek people who think in code and are motivated by being around like-minded people.
The Role:

Participate in the development and maintenance of our large-scale, distributed storage platform.
Interface with the users of the system to understand their use-cases and assist them in making the best use of our tools and platforms
Research best practices and propose feature specifications for our platform roadmap
Develop and maintain features and tools that complement our core platform offering

What You'll Bring:

University Degree, preferably in Computer Science, Electrical Engineering or related fields.
At least 5 years of work experience.
Strong (English) communications skills and experience working in a global environment
Expertise in Software Life Cycle management methodologies and tools, including Git and JIRA (required)
Expertise in C/C++, and/or Go and/or Python (at least one required)
UNIX/Linux (required).
Knowledge on the design and development of large-scale, distributed storage systems (desirable)

What We Offer:

Competitive compensation package
Core benefits include: premium private health insurance and life insurance with savings plan
Support for every aspect of life through Employee Assistance Program and fully covered sick leave
Strong culture of learning and development: training courses, library, guest speakers, share and learn events, global conferences
Regular offsite team buildings, annual conferences and occasional global summits – opportunity to travel and connect with our local and global teams


#LI-MH1By submitting this application, you acknowledge and consent to terms of the WorldQuant Privacy Policy. The privacy policy offers an explanation of how and why your data will be collected, how it will be used and disclosed, how it will be retained and secured, and what legal rights are associated with that data (including the rights of access, correction, and deletion). The policy also describes legal and contractual limitations on these rights. The specific rights and obligations of individuals living and working in different areas may vary by jurisdiction.
Copyright © 2025 WorldQuant, LLC. All Rights Reserved.WorldQuant is an equal opportunity employer and does not discriminate in hiring on the basis of race, color, creed, religion, sex, sexual orientation or preference, age, marital status, citizenship, national origin, disability, military status, genetic predisposition or carrier status, or any other protected characteristic as established by applicable law.`,
			},
			{
				Path:        "senior-backend-engineer-ahsd1",
				URL:         "https://www.cake.me/companies/wit-software-ltd/jobs/senior-backend-engineer-ahsd1",
				Title:       "Senior Backend Developer / 資深後端工程師 ( 轉 Golang )",
				Description: `▎ What You'll Do1. 掌握現有架構，協助架構優化與技術選型。( e.g. 效能瓶頸、系統擴展、資料存儲, etc. )2. 參與開發，為產品推出更多全新功能。( e.g. 智能回覆、匿名聊天, etc. )3. 帶領團隊成員分析線上產品數據。( e.g. network traffic, bandwidth, memory, and storage estimates )4. 持續學習新技術、主動分享並嘗試將適合的技術導入。【 In the first month 】◾️ 了解團隊目標、目前產品和未來規劃◾️ 搭建自己的工作環境，掌握當前的分散式架構 ( e.g. 傳輸/編碼協定、服務框架、業務邏輯, etc. )◾️ 掌握目前使用的相關技術 ( e.g. Redis, MySQL, MongoDB, Elasticsearch etc. )◾️ 掌握當前資料處理的設計目的 ( e.g. DB Schema, Data flow, etc. )【 In the first three months 】◾️ 開始參與需求 (e.g. 新產品、功能擴充/變更, etc.) 討論, 並確實完成交付◾️ 分析系統瓶頸，提出可被實踐的解決方案 ( e.g. RCA → 測試報告 → 解決方案 → 加入排程 )【 In the first six months, expect to 】◾️ 擔任 Mentor 帶領團隊其他成員◾️ 了解產品下一階段目標，事前規劃協助推動產品迭代◾️ 與 Team Lead 討論下一階段的職涯規劃`,
			},
			{
				Path:  "pure-remote-t-multinational-game-company-is-seeking-go-business-development-developers",
				URL:   "https://www.cake.me/companies/interislandtw/jobs/pure-remote-t-multinational-game-company-is-seeking-go-business-development-developers",
				Title: "【純遠端 T】跨國遊戲公司 誠徵 Go工程師",
				Description: `1、負責GRPC和大廳業務；
2、負責需求評審、大廳框架搭建、規劃和設計；負責解決方案、架構優化方案；
3、負責代碼編寫、規範和標準設定；
4、負責大資料、高併發、高承載設計；
5、負責遊戲、休閒遊戲維護反覆運算升級
6、負責公司後端架構的搭建和優化，持續優化服務的可用性、伸縮性、穩定性；`,
			},
			{
				Path:  "software-engineer-virtual-insurance",
				URL:   "https://www.cake.me/companies/aift/jobs/software-engineer-virtual-insurance",
				Title: "Software Engineer(Golang, Flutter), Virtual insurance",
				Description: `About the Role
We are building a Pet Ecosystem that integrates insurance, pet health, and IoT—serving real user needs at scale. This role is not just about shipping features. You will design systems, influence product decisions, and help define how technology drives growth and user experience.
 
 Responsibilities
1. Build Scalable Systems  Own Delivery
Design and deliver production-grade APIs and services powering a fast-growing mobile app Own end-to-end features—from system design to implementation and optimization Drive architecture decisions across microservices, scalability, and reliability 
2. Engineer for Performance  Reliability
Build systems that handle real-world traffic, latency, and edge cases Continuously improve performance (API, database, system throughput) Establish strong engineering foundations: testing, CI/CD, observability, and security 
3. Shape Product Through Technology
Partner closely with Product, Design, App, Data and BE teams to define solutions—not just implement them Enable data-driven decisions through tracking, experimentation, and analytics Support integrations across mobile and IoT, turning real-world signals into product value How to apply
Please apply this position through 👉 https://grnh.se/lk9q1h574usIt will help us process your applications faster!`,
			},
			{
				Path:        "asia-t-multinational-gaming-company-is-seeking-a-senior-executive-secretary",
				URL:         "https://www.cake.me/companies/interislandtw/jobs/asia-t-multinational-gaming-company-is-seeking-a-senior-executive-secretary",
				Title:       "【日本】遊戲公司 誠徵 HR、UI、遊戲動效、 Cocos前端、Go後端、機率工程師、遊戲製作人",
				Description: `一、HR 東京工作內容人才獲取 : 負責東京研發團隊的端到端招聘。制度搭建: 根據日本勞動基準法，從0到1建立和完善公司的人事規章制度（就業規則）、薪資福利系統及員工手冊。員工關係與社保: 負責入職離職手續、簽證辦理支援、社會保險繳納及每月薪資核算（協同外部稅理士/勞務士）。職務需求語言能力： 中文與日語皆達到商務流利程度（需以工作語言進行書面與口頭溝通）。教育背景： 大學或以上學歷。從業經驗： 3-5年以上日本本土HR實務經驗，熟悉日本勞動法及相關政策。產業偏好： 具備網路遊戲產業招募經驗者優先。核心能力：具備極強的自驅力和適應力，能夠接受團隊初期的不確定性。二、UI設計工作內容1. 根據企劃獨立完成不同主題、文化的遊戲概念設計。2. 能夠主導推進遊戲圖示、角色、場景、以及UI介面設計，宣傳圖繪製及優化宣傳素材。3. 對slots機台的美術製作流程有基本的瞭解和管理能力，能夠制定大體的製作計畫。4. 根據團隊和決策者的回饋，不斷改進和完善設計，積極參與協調美術和開發的對接工作，解決遊戲開發流程中的美術問題。職務需求：1. 大學及以上美術、設計相關科系畢業。2. 熟練掌握歐美棋牌或風格（類Cash Frenzy、Cash Carnival、WSOP、Zynga Poker），同時掌握歐美寫實和歐美卡通類風格者佳。3. 有動畫、動效設計製作經驗者優先。三、遊戲動效職務內容：1. 製作符合遊戲標準的高品質特效（動畫效果），提出遊戲及功能的視覺風格創意。2. 與UI設計師及產品負責人合作，製作角色、物品、場景及UI的特效。3. 將動畫匯出並進行優化以供遊戲使用，並與程式設計師合作，確保在技術限制範圍內保持動畫品質。4. 與團隊共同集思廣益，提出新的遊戲創意。職務需求：1. 大學以上學歷，美術相關專業畢業者優先。2. 熟悉Unity引擎和常用的Unity外掛。3. 熟悉AE、3ds Max、Photoshop等軟體。4. 在3ds Max中具備基本的製作簡單動畫能力。5. 具備扎實的美術基礎與基本的手繪能力。6. 具備1年及以上相關工作經驗，有歐美風格特效製作經驗者優先考慮。7. 請在履歷中附上個人作品或聯繫方式，發送至指定的郵件地址。• 8. 給加分項：掌握主流的AI影片軟體，能夠用於實際工作中提升工作效率和品質者優先考慮。四、前端 Cocos 工程師職務內容：1、負責遊戲功能設計開發，確保項目進度按時進行；2、負責線上項目維護，bug查改；3、負責各種平台（iOS、Android）的移植、SDK接入、適配、打包、發佈。職務需求：1、大學及以上學歷，電腦、電子等相關專業優先；2、3年及以上Cocos 開發經驗，有cocos creator開發經驗優先；3、熟悉常用數據結構、演算法、物件導向程式設計思想和設計模式；4、有良好的溝通能力與團隊協作能力，較高的工作責任心和敬業精神；動手能力強，有解決複雜問題的能力與興趣。五、後端 GO工程師職務內容- 設計、開發和維護高可用性和高併發的遊戲伺服器架構和邏輯。- 負責遊戲核心業務邏輯的實現，例如帳號管理、戰鬥運算、物品交易等。- 進行伺服器效能調優、故障排除，確保遊戲服務的穩定運行。- 設計和實施資料庫結構，優化資料存取和儲存效率。- 與遊戲客戶端開發團隊緊密合作，制定高效的網路通訊協議。職務需求：- 精通Go，並有實際專案經驗。-大學及以上學歷，電腦、電子等相關專業優先；- 熟悉網路編程（TCP/IP, Socket）和多執行緒/多程序並發處理機制。- 熟悉 MySQL/NoSQL 等資料庫操作和優化。- 具備大型分散式系統設計、負載平衡或高併發處理經驗者優先。- 對遊戲的業務邏輯有熱忱，能從伺服器角度理解遊戲設計。六、機率工程師職務內容1. 協助遊戲的整體企劃與製作流程管理，包含專案規劃、時程控管與里程碑設定，確保專案如期且高品質上線。2. 協助遊戲玩法核心邏輯設計，尤其著重於 Slot 機台的數學模型、獎勵機制與遊戲節奏體驗。3. 與美術、程式、音效、數學、資料分析及在地化團隊密切合作，確保遊戲內容符合市場需求與產品定位。4. 研究目標市場與競品動態，並依據數據分析結果持續優化遊戲表現。任職條件1. 大學以上學歷，數學、統計或相關科系尤佳。2. 具備已上線專案之 數值／機率系統設計經驗。3. 對遊戲數值具備良好敏感度，熱衷研究線上遊戲之數值系統與相關數學模型。4. 能獨立使用數學與分析工具（如 Excel、MATLAB、Python 等）建立大型數值模型。5. 具備良好的團隊合作精神與溝通協調能力，能適應較高工作強度與專案壓力。七、遊戲製作人職務內容 1. 專案管理與整體規劃負責 Slots 類型遊戲之整體企劃與製作流程管理，制定專案計畫、時程與里程碑，確保產品如期且高品質上線。 2. 玩法與系統設計主導遊戲核心玩法設計，包含 Slot 機台之數學模型、獎勵機制、遊戲節奏與整體玩家體驗。 3. 跨部門溝通協作與美術、程式、音效、數學、數據分析及在地化團隊密切合作，確保遊戲內容符合產品定位與市場需求。 4. 市場導向與產品優化研究目標市場及競品動態，透過數據分析持續優化遊戲表現與玩家留存。 5. 團隊領導與人才培育指導並培養團隊成員，建立高效的開發流程與正向創意文化。任職條件1. 學歷與經驗 • 大學（含）以上學歷，遊戲設計、數學、統計、資訊工程或相關科系尤佳 • 具 3 年以上 Slots 遊戲開發經驗，曾獨立主導至少 1 款已上線產品2. 專業能力 • 熟悉 Slots 遊戲核心玩法與相關數學模型 • 具備扎實的專案管理能力與實務經驗 • 熟悉至少一種主流遊戲引擎 • 對數據分析與玩家心理有深入理解3. 個人特質 • 具備良好溝通與協調能力，能有效整合跨部門資源推動專案進行 • 富有創意與市場敏感度，能掌握競品趨勢與玩家行為 • 具結果導向思維，抗壓性高，能適應快速變動的工作節奏4. 加分條件 • 具線上 Casino 或 Social Casino 產品開發經驗 • 熟悉北美市場 Slots 產品型態 • 具數學模型相關背景或實務經驗`,
			},
			{
				Path:  "senior-backend-engineer-php-golang-node-js-multi-product-saas-platform",
				URL:   "https://www.cake.me/companies/morgan-philips-e5774d/jobs/senior-backend-engineer-php-golang-node-js-multi-product-saas-platform",
				Title: "Senior Backend Engineer (PHP / Golang / Node.js) |  Multi-Product SaaS Platform",
				Description: `Our client is a fast-growing consumer technology company with a strong focus on digital commerce, product innovation, and scalable service platforms. As the business continues to grow, the engineering team is facing increasing technical challenges across high-traffic services, cloud architecture, rapid feature iteration, internal systems, and data-driven product development.

The company maintains a collaborative and agile engineering culture, where engineers are encouraged to solve real business problems through practical system design, clean code, data-driven decision-making, and continuous technical improvement.
About the Role
The company is looking for a Senior Backend Engineer to join its engineering team in Hsinchu. This role will be responsible for backend system design, application development, database schema design, internal system development, and architecture optimization.
The ideal candidate should be comfortable working on both product-facing and internal systems, with the ability to evaluate technical trade-offs, write maintainable code, and contribute to scalable backend architecture.`,
			},
			{
				Path:  "leader-tiktok-go",
				URL:   "https://www.cake.me/companies/pt-entropi-global-martech/jobs/leader-tiktok-go",
				Title: "Leader Tiktok Go",
				Description: `📝 Job Description:- Memimpin dan mengelola seluruh tim TikTok Go- Menyusun strategi bisnis dan pengembangan divisi- Mengontrol pencapaian KPI dan target revenue- Membuat perencanaan, monitoring, dan evaluasi performa tim- Memberikan arahan, coaching, dan problem solving- Bertanggung jawab atas pertumbuhan dan profitabilitas divisi
`,
			},
			{
				Path:  "backend-engineer-42f",
				URL:   "https://www.cake.me/companies/7b9/jobs/backend-engineer-42f",
				Title: "後端工程師(golang)",
				Description: `艾捷科技有限公司致力於推動企業 IT 基礎架構的最佳體驗！我們是專業的 IT 諮詢服務與企業基礎建設解決方案提供商，如果你期待成為一個技術與創意結合的團隊的一員，這裡將是實現自我價值的理想舞台✨！
【工作內容】1. 現有專案/新專案的後端開發及維護2. 依據回饋改善及提高使用者體驗3. 評估並運用新技術，優化工作流程及開發模式4. 規劃系統架構並實作，保持系統可靠性、可擴展性、可維護性5. 具良好的計劃和管理能力，能根據協議要求提供穩定、可靠的監控服務6. 具良好的分析和溝通能力，能處理監控警報產生的相關問題，並協調解決方案7. 其他主管交辦事項
`,
			},
			{
				Path:  "senior-developer-for-red-team-tool",
				URL:   "https://www.cake.me/companies/devcore/jobs/senior-developer-for-red-team-tool",
				Title: "Senior Developer 紅隊工具開發工程師",
				Description: `你將與台灣最頂尖的資安專家並肩作戰，負責開發及維護紅隊演練工具。你參與的專案幾乎是外面不會有的需求，適合熱愛技術挑戰的你。你可以直接面對使用者討論，通常使用者也能直接給予技術回饋甚至方案建議。團隊成員對自己的作品充滿認同與自豪，若你也熱衷技術、追求卓越、重視團隊，歡迎加入我們！下面是工作內容：
開發資安檢測相關工具 80%包含但不限於遠端控制、網路傳輸、提權等檢測過程中所需工具改進、擴充現有滲透工具理解團隊需求 20%能理解紅隊思維與團隊作戰需求，轉換成對應開發項目配合團隊需求，評估時程提出解決辦法，並主導開發
履歷內容請務必控制在兩頁以內（超過兩頁將直接視為資格不符），並且至少須包含以下內容：
基本資料學歷工作經歷MBTI 職業性格測試結果（測試網頁）若您願意提供 MBTI 測驗結果，可讓我們更瞭解您偏好的溝通模式，若您不願意提供，也不影響本次審核結果。`,
			},
			{
				Path:  "backend-engineer-5da",
				URL:   "https://www.cake.me/companies/412/jobs/backend-engineer-5da",
				Title: "後端工程師 (Golang/Go)",
				Description: `負責核心後端服務與微服務架構的設計、開發與優化使用 Go（Golang）打造高效能、高可用且具擴展性的系統深度參與系統架構演進，支撐產品在高併發與大規模使用情境下穩定運行與產品、前端及 DevOps 團隊協作，確保服務品質與交付效率如果你熱衷於打造高效能分散式系統，並希望在微服務與雲端架構中發揮技術影響力，歡迎加入我們！主要職責端開發與系統架構


使用 Golang 設計與開發高效能、可擴展的微服務系統


參與系統架構設計與演進（Microservices、Service-to-Service 通訊、API Contract）


設計模組化、可維護的程式結構，確保長期可擴展性


可觀測性與系統穩定性


建立與維護系統監控、日誌與分散式追蹤機制


使用 OpenTelemetry（OTEL）導入 metrics、logs、traces


協助問題排查（debugging）與效能瓶頸分析


資料庫與資料處理


設計與優化資料模型，支援高效能資料存取


操作關聯式資料庫（PostgreSQL、MySQL）與快取／NoSQL（Redis 等）


進行查詢優化、索引設計與資料一致性管理


API 設計與系統整合


設計與實作高可用 API（RESTful / gRPC）


確保服務間通訊穩定、具擴展性與良好錯誤處理機制


與前端及第三方服務進行整合


測試與效能優化


撰寫單元測試、整合測試與效能測試，提升系統可靠性


分析系統效能（CPU、Memory、Latency），持續優化服務表現


解決高併發場景下的穩定性與資源使用問題


DevOps 與部署協作


與 DevOps 團隊合作，透過 Docker、Kubernetes 部署服務


支援 CI/CD 流程，提升開發與發布效率


熟悉雲端環境（AWS、GCP）服務運作`,
			},
			{
				Path:  "senior-golang-engineer-8a6",
				URL:   "https://www.cake.me/companies/bitogroup/jobs/senior-golang-engineer-8a6",
				Title: "Senior Backend Engineer (Golang)（每月有遠端日）",
				Description: `團隊介紹

Golang 團隊主要負責交易撮合以及區塊鏈錢包等功能，除了業務邏輯的開發維護，我們也會跟 SRE 合作導入新技術以及優化架構，例如為了提升系統可觀測性，埋入更多 promethues metrics 以及導入 opentelemtry。


身為 Golang 團隊的一員，你會感受到我們的團隊文化是：

比起單打獨鬥，我們更重視主動的溝通，在每日站立時，大家會清楚地說明負責的工作項目內容，目的是讓團隊能清楚彼此的工作範圍，提升整體對系統的熟悉度，也能互相提醒注意事項。
比起定義 Deadline，我們更重視定義明確的 Delivery，Sprint 開始前，大家會明確定義這次 Sprint 的產出，在 Sprint 的結束會彼此分享並 Demo 產出的內容，目的是每次 Sprint 聚焦工作目標，讓工作成果能有節奏地累積。
比起 Top-Down 指配任務，我們更重視雙向的討論，我們會定義每季的 OKR，目的是讓組員能清楚知道每個專案對公司的價值，也給組員機會向上提供專案建議，建立個人目標與公司目標的關聯。
比起沈默不語，我們更重視有火花的溝通，鼓勵大家表達想法，擁抱不同的意見，雖然溝通有時會耗費心力與時間，但溝通能幫助我們凝聚共識，瞭解彼此，提升合作默契，在我們團隊中，溝通的準則是先對齊情境與問題，在討論方法與實作。
比起馬上寫扣，我們更重視先做好系統分析，每個專案開始前都會透過系統分系，定義清楚專案的實作內容以及修改範圍，避免專案做到一半，發現要重構或方向錯誤，也避免專案做完發現沒兼顧可擴充的特性。
比起這樣就好，我們更重視深度地討論，我們明白專案會有時程的壓力，我們不追求實作能立馬完美，但期望在系統分析的過程中，能有深度地討論，能清楚明白當前設計的風險以及取捨。
比起一成不變，我們更重視調整與進步，我們是扁平的團隊，有任何想法都會被提出並討論，除了專案之外我們也會討論工作合作方式，例如 Code Review 機制，PR 顆粒度，如何回收技術債等等，沒有人是完美的，我們期待互相激勵，促進反思進步。
比起工作關係，我們更重視夥伴關係，除了工作專案交流，我們平常也會交流生活，每月聚餐，講幹話，討論職涯，討論如何發財，討論新技術等等。



工作內容

身為團隊的 Senior 工程師，我們期待你：

能成為專案的負責人，主動與 PM 釐清模糊的需求，主持系統分析會議，切分專案的實作內容，協助成員理解專案，分配任務，擔任專案主要窗口，與其他團隊說明 API 串接方式，規劃專案 Release 以及壓測計畫等。
能主動發掘系統中待優化的地方，並在團隊會議主動提出並主持討論，協助其他成員理解問題脈絡，一起討論優化方案，主動規劃落地方案。
在不同專案的系統分析中，不僅能在業務邏輯中給予設計上的想法，也能在架構層提供精準的建議，例如 index 的設計，query 的使用，redis 結構設計如何降低記憶體使用率等。
能幫助團隊在文化上，開發流程以及 best practice 不斷地迭代改進。
對團隊與專案擁有責任感，面對問題能主動釐清，協助他人找到解法，面對複雜的問題，要有找不到 Root Cause 不善罷甘休的氣勢。
要有能力清楚且有脈絡地表達與論述事情。
對於溝通富有耐心，可以堅持立場，但不會輕易爆氣，期望你能指導其他工程師，能主動分享技術知識。
提供高品質的程式，例如清楚的命名，好維護的封裝，可測試的程式，清楚易用的介面，內聚高的業務模組。
與不同團隊與部門能緊密的合作，例如與 PM 合作專案，與前端合作 API 串接，與 QA 合作自動化測試的建置，與 SRE 合作導入新的架構與工具。


進入後


1個月內，你將會：

了解團隊開發流程
了解團隊開發 Convention
建制開發環境
透過新手專案，熟悉 codebase 以及開發流程
修復一個技術債
了解不同專案的 Scope 以及參與系統分析

3個月後，你將會：

解決更複雜的架構性技術債
負責交易所交易核心以及 API 優化與維護
主導或參與更多專案
參與團隊 OKR




`,
			},
			{
				Path:  "excellent-salary-and-benefits-engineering-manager-backend-lead-golang-core-system-deve-90e",
				URL:   "https://www.cake.me/companies/cake-recruitment-consulting/jobs/excellent-salary-and-benefits-engineering-manager-backend-lead-golang-core-system-deve-90e",
				Title: "🌍 薪資福利優渥 - Sr. Backend Engineer(Golang, Node.js, Java) to Tech Lead｜AI 應用核心系統開發 - Candice",
				Description: `我們正在尋找一位想主導架構、影響產品方向、打造可長期演進的系統的優秀人才設計與優化 高可用、高效能、可擴展 的後端系統架構 

使用 Golang / Python / Java 打造核心服務（依產品場景選擇最適語言）


主導關鍵技術決策（架構、技術選型、系統拆分、效能策略）


與 Product / Frontend / Data / DevOps 深度協作，讓想法真正落地
`,
			},
			{
				Path:  "gip",
				URL:   "https://www.cake.me/companies/hennge/jobs/gip",
				Title: "Software Engineer Intern (Onsite; Tokyo)",
				Description: `About the job
HENNGE’s Global Internship Program (GIP) is an on-site software engineering training program designed and run by HENNGE, a cloud security company based in Tokyo. The program is created for students and early-career engineers who want to strengthen their engineering skills in a global, English-speaking environment at HENNGE’s Shibuya office. The program is on-site at HENNGE’s Shibuya office (Mon–Fri, 10:00–19:00 JST).

We currently have two pathways available:

Frontend Pathway: Batch 5 (2026): Nov 16 - Dec 18 (5 weeks)Full-Stack Pathway: Batch 1 (2027): Jan 18 - Feb 19 (5 weeks)
Can't find a time that fits your schedule? We open five batches a year! Sign up for our newsletter to get the latest information here. 

---

The Full-Stack Pathway focuses on hands-on application development training across back-end, front-end, and basic DevOps, using Python (non-ML stack), Go, or TypeScript on the back-end and TypeScript with modern front-end frameworks on the front-end. The program emphasizes fundamental engineering skills, problem-solving, explanation, and collaboration, rather than production work.
What You’ll Learn  Practice

During the program, you will:
Develop full-stack web applications as part of structured training projectsBuild and connect back-end APIs and services with front-end applicationsPractice key full-stack engineering concepts, including:Back-end logic and API designFront-end state management and data flowAuthentication and security basicsSoftware testing (unit, integration, end-to-end)Learn, at a practical level, how front-end, back-end, and cloud infrastructure interactWork with Unix-like environments and basic DevOps toolingExplain your technical decisions, trade-offs, and problem-solving approach during discussions and reviewsReceive feedback from mentors and iteratively improve your implementation
---

The Front-End Pathway focuses on hands-on application development training using TypeScript and modern front-end frameworks (such as React or Vue), with mentorship from HENNGE’s experienced engineers. The program emphasizes self-learning, explanation, and collaboration rather than production work.
What You’ll Learn  Practice

During the program, you will:
Develop front-end applications as part of structured training projects using TypeScript with React or VuePractice key front-end engineering concepts, including:Form validationData fetchingState managementLearn, at a practical level, how front-end applications interact with back-end services and cloud infrastructureExplain your technical decisions, trade-offs, and problem-solving approach during discussions and reviewsReceive feedback from mentors and improve your implementation iteratively

---
Benefits 
Round-trip airfares from and back to your country of residencePre-arrival support and visa guidanceMedical insurance* and a phone with mobile data will be provided during your internship in Tokyo, Japan**Up to 150,000 JPY of monthly subsidy to cover accommodation, meals, transportation, and networking activitiesSubsidized company events on top of monthly subsidy (Monthly Technical Sessions, Board Game Night, Get-to-know-you Lunches, and many others!) to get to know teams across divisionsAccess to online learning platform during the internship period
* Medical insurance will only be provided to interns who do not reside in Japan
** This internship is an unpaid training program with a subsidy provided to all interns
`,
			},
			{
				Path:        "web3-blockchain-new-team-backend-engineer",
				URL:         "https://www.cake.me/companies/cycatena/jobs/web3-blockchain-new-team-backend-engineer",
				Title:       "後端工程師｜Backend Engineer（區塊鏈 Web3 公司）",
				Description: `協助開發 Web3 安全相關產品研究新技術，導入並優化現有產品集思廣益，讓產品更好依照需求開發後端相關功能`,
			},
			{
				Path:  "832c3d",
				URL:   "https://www.cake.me/companies/mowaytech/jobs/832c3d",
				Title: "Golang 工程師 (台中)",
				Description: `▌工作內容
高效能系統設計與開發：大型網路平台的核心功能建構。運用 Go 語言的優勢，打造高穩定、可擴展且易於維護的後端服務，確保系統在面對高頻存取時依然能運作自如。
API 架構維護：負責高效能 API的規劃與迭代，透過持續的維護與優化，提供前端夥伴最堅實的支撐。
跨團隊技術協作與專案推進：深度參與專案從 0 到 1 的生命週期。與產品、前端及運維團隊展開技術對話，在討論中尋求效能與開發速度的最佳平衡點。
技術文件建置：負責架構與開發流程的文件化，透過完整的文件撰寫，讓團隊知識得以傳承，並降低溝通成本。`,
			},
			{
				Path:  "high-concurrency-fintech-systemsr-backend-engineer-java-welcome-to-convert-to-golang-and-c",
				URL:   "https://www.cake.me/companies/cake-recruitment-consulting/jobs/high-concurrency-fintech-systemsr-backend-engineer-java-welcome-to-convert-to-golang-and-c",
				Title: "💼【高併發 Fintech 系統】Sr. Backend Engineer（Java／歡迎 Golang、C# 轉換）-TL",
				Description: `
我們是一家國際級 Fintech 團隊，擁有超過百萬用戶的交易平台，致力於打造穩定、安全且快速的數位資產應用。這裡是工程師發揮技術深度與國際影響力的絕佳舞台。
我們的技術文化講求 Ownership、創新與自動化，團隊扁平、溝通直接，對工程品質與穩定性高度重視。若你喜歡用技術解決挑戰、參與交易核心系統建置，這將是你發光的舞台。
💡【你將負責】我們是一間專注於金融科技與區塊鏈的國際團隊，提供交易平台與數位資產管理解決方案。現正擴編核心工程團隊，歡迎對高效能後端系統開發有熱情的工程師加入。
💡【你將負責】


使用 Java 開發與維護高併發、高穩定性的交易應用


設計資料結構與商業邏輯，撰寫清晰 API 文件


與產品、設計與資深開發團隊合作，迭代優化功能


`,
			},
			{
				Path:  "senior-backend-engineer-virtual-insurance",
				URL:   "https://www.cake.me/companies/aift/jobs/senior-backend-engineer-virtual-insurance",
				Title: "Senior Backend Engineer (Golang), Virtual insurance",
				Description: `Responsibilities
Design, build, and maintain backend services for B2C Insurance/IoT/Shopping/Point/Payment systems.Build the APIs and design the data schema to fulfill the business requirements.Write tests and constantly seek to improve code quality and reliability.Drive the quality standards within the development team by example, produce highly usable technical documentation as well as conduct code reviews.Collaborate with cross-functional teams to deliver end-to-end solutions.`,
			},
			{
				Path:        "senior-golang-engineer-6f8",
				URL:         "https://www.cake.me/companies/d38998/jobs/senior-golang-engineer-6f8",
				Title:       "資深Golang工程師",
				Description: `■ 薪資條件：60,000~200,000 NTD■ 工作時間：9:00~18:00需求分析與後端架構設計及開發：依據產品需求，使用Golang語言設計和改進系統架構，確保系統的可擴展性、穩定性和性能API Server的開發與串接：設計和實現RESTful，並確保API的可靠性和安全性性能優化：不斷優化程式碼效能、實現高品質且可重複利用的程式碼，並提升系統的響應速度和處理能力問題排查：處理各類軟體技術問題與 Bug 修正跨部門協作：參與跨部門合作，與團隊成員協作配合，共同開發和解決技術問題件`,
			},
		},
	}
	assert.Equal(t, wantResp, &got)
}

func TestCakeSearchJobsMissingRequiredE2E(t *testing.T) {
	clientSession, _ := testCakeMCPClientServer(t)

	// Missing required params are rejected by the SDK's input-schema
	// validation before the handler runs, as an IsError tool result.
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no keyword", map[string]any{"location": "台灣"}, `validating "arguments": validating root: required: missing properties: ["keyword"]`},
		{"no location", map[string]any{"keyword": "Golang"}, `validating "arguments": validating root: required: missing properties: ["location"]`},
		{"empty args", map[string]any{}, `validating "arguments": validating root: required: missing properties: ["keyword" "location"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "cake_search_jobs",
				Arguments: tc.args,
			})
			require.NoError(t, err)
			require.True(t, callRes.IsError)
			text, ok := callRes.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Equal(t, tc.want, text.Text)
		})
	}
}

func TestCakeSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testCakeMCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError
	// tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "cake_search_jobs",
		Arguments: map[string]any{"keyword": "Golang", "location": "台灣", "job_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, `validating "arguments": validating root: validating /properties/job_type: enum: valueNotInEnum does not equal any of: [full_time part_time internship contract freelance temporary volunteer]`, text.Text)
}

func TestCakeGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testCakeMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "cake_get_job_detail",
		Arguments: map[string]any{"path": "senior-golang-web-backend-engineer-taoyuan"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got cakeDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want := &cakeDetailOutput{
		ID:       46590,
		Path:     "senior-golang-web-backend-engineer-taoyuan",
		URL:      "https://www.cake.me/companies/lctech_/jobs/senior-golang-web-backend-engineer-taoyuan",
		PagePath: "lctech_",
		Title:    "資深 Golang 網頁後端工程師(桃園)",
		Description: `【職務內容】

維護現有系統與功能、新專案開發、跨部門團隊需求

標配MAC電腦

(本職務辦公室位於桃園，遠端工作需求面議)

【上班時間】

早上 9 點至下午 6 點並且有 30 分鐘彈性上下班

【公司福利】

勞健保、三節、尾牙、生日禮金。

我們公司設有咖啡休息區，免費零食、飲料、餐點，飲品，備有微波爐。

不定時員工聚餐與每年都有員工旅遊。`,
		Requirements: `** *【基本條件】*

※ 應徵者務必附上作品或相關連結

※ 溝通上有耐心、邏輯思路清晰、團隊合作

※ 有耐心 troubleshooting

※ 理解通達 Go 1.23 以上

※ 理解通達 Protocol Buffers, Proto Validation, gRPC

※ 理解通達 HTTP/1.1, HTTP/2, HTTP/3

※ 理解通達 SA / SD, Event Sourcing, Hexagonal Architecture, Saga Pattern

※ 理解通達 PostgreSQL, Redis, Transaction processing

※ 理解通達 Github, Gitflow, Trunk-based workflow

※ 理解通達 Unit Testing, Integration Testing, Mock Testing, Load Testing

※ 有耐心與商務團隊協作以及提供支援

※ 理解通達產品研發迭代的工作模式

※ 有耐心參與團隊事務互動以推動整體團隊工作

※ 能夠依據商業需求驅動開發工作

※ 理解通達職場禮節(workplace etiquette)

※ 能提出有效系統設計方案

※ 能提升開發小組整體品質與效率且降低開發成本

※ 能最佳化整體系統資源利用率

※ 完成主管交辦事項

*【加分條件】*

※ 對 DDD、BDD、TDD、Gherkin 有一定程度認知

※ 對主流市場、產品、設計具有強烈方向感

※ 對專業技術具高度敏感與高度自主學習力

*【什麼人適合我們】*

為人誠信、態度認真負責、充滿熱情與理想、擁有毅力與耐力，出眾的專業能力，歡迎來公司聊聊，期待你的加入！

(年薪固定或變動薪資因個人資歷或績效而異)`,
	}
	assert.Equal(t, want, &got)
}

func TestCakeHTTPToMCPResponse(t *testing.T) {
	in := cake.JobSearchResponse{
		TotalEntries: cake.NewNilInt(2),
		TotalPages:   cake.NewNilInt(1),
		PerPage:      cake.NewNilInt(20),
		CurrentPage:  cake.NewNilInt(1),
		Data: []cake.JobSearchItem{
			{Path: "p1", Title: cake.NewNilString("t1"), Description: cake.NewNilString("d1"), Page: cake.NewOptJobSearchPage(cake.JobSearchPage{Path: cake.NewNilString("pp1")})},
			{Path: "p2", Title: cake.NewNilString("t2"), Description: cake.NewNilString("d2")},
		},
	}
	got := cakeHTTPToMCPResponse(&in)

	want := &cakeSearchOutput{
		TotalEntries: 2,
		TotalPages:   1,
		PerPage:      20,
		CurrentPage:  1,
		Data: []cakeJobSummary{
			{Path: "p1", URL: "https://www.cake.me/companies/pp1/jobs/p1", Title: "t1", Description: "d1"},
			{Path: "p2", Title: "t2", Description: "d2"},
		},
	}
	assert.Equal(t, want, got)
}

func TestCakeHTTPToMCPDetail(t *testing.T) {
	in := cake.JobDetail{
		ID:           cake.NewNilInt(7),
		Path:         cake.NewNilString("p"),
		PagePath:     cake.NewNilString("pp"),
		Title:        cake.NewNilString("t"),
		Description:  cake.NewNilString("<p>d</p>"),
		Requirements: cake.NewNilString("<p>r</p>"),
	}
	got := cakeHTTPToMCPDetail(&in)

	want := &cakeDetailOutput{
		ID:           7,
		Path:         "p",
		URL:          "https://www.cake.me/companies/pp/jobs/p",
		PagePath:     "pp",
		Title:        "t",
		Description:  "d",
		Requirements: "r",
	}
	assert.Equal(t, want, got)
}

func TestCakeMCPToHTTPRequest(t *testing.T) {
	in := cakeSearchInput{
		Keyword:   "golang",
		Location:  "Taiwan",
		JobType:   "part_time",
		Seniority: []string{"mid_senior_level", "director"},
		Remote:    "full_remote_work",
		Sort:      "latest",
		Page:      2,
	}
	got, err := cakeMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &cake.JobSearchRequest{
		Query:  "golang",
		Page:   cake.NewOptInt(2),
		SortBy: cake.JobSearchRequestSortByLatest,
		Filters: cake.JobSearchFilters{
			Locations:       []string{"Taiwan"},
			JobTypes:        []cake.JobSearchFiltersJobTypesItem{cake.JobSearchFiltersJobTypesItemPartTime},
			SeniorityLevels: []cake.JobSearchFiltersSeniorityLevelsItem{cake.JobSearchFiltersSeniorityLevelsItemMidSeniorLevel, cake.JobSearchFiltersSeniorityLevelsItemDirector},
			Remote:          []cake.JobSearchFiltersRemoteItem{cake.JobSearchFiltersRemoteItemFullRemoteWork},
		},
	}
	assert.Equal(t, want, got)
}

func TestCakeMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := cakeMCPToHTTPRequest(&cakeSearchInput{Keyword: "golang", Location: "台灣"})
	require.NoError(t, err)

	// The Cake API requires sort_by, so the converter defaults it to
	// popularity when the tool input omits sort.
	want := cake.JobSearchRequest{
		Query:  "golang",
		SortBy: cake.JobSearchRequestSortByPopularity,
		Filters: cake.JobSearchFilters{
			Locations: []string{"台灣"},
		},
	}
	assert.Equal(t, want, *got)
}

func TestCakeMCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   cakeSearchInput
		want string
	}{
		{"all empty", cakeSearchInput{}, "keyword is required"},
		{"filters only", cakeSearchInput{Location: "Taiwan", Sort: "latest", Page: 2}, "keyword is required"},
		{"keyword only", cakeSearchInput{Keyword: "golang"}, "location is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cakeMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestCakeMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   cakeSearchInput
		want string
	}{
		{"job_type", cakeSearchInput{Keyword: "x", Location: "Taiwan", JobType: "Full-time"}, `invalid job_type "Full-time"`},
		{"seniority", cakeSearchInput{Keyword: "x", Location: "Taiwan", Seniority: []string{"mid_senior_level", "staff"}}, `invalid seniority "staff"`},
		{"remote", cakeSearchInput{Keyword: "x", Location: "Taiwan", Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"sort", cakeSearchInput{Keyword: "x", Location: "Taiwan", Sort: "newest"}, `invalid sort "newest"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cakeMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
