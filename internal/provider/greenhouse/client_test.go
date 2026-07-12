package greenhouse

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{BoardToken: "safariai"})
	require.NoError(t, err)

	got, ok := res.(*JobListResponse)
	require.True(t, ok, "want *JobListResponse, got %T", res)

	assert.Equal(t, &JobListResponse{
		Meta: NewOptJobListResponseMeta(JobListResponseMeta{Total: NewOptInt(7)}),
		Jobs: []JobSummary{
			{
				ID:                  NewOptInt(5019966007),
				InternalJobID:       NewOptNilInt(4577205007),
				Title:               NewOptNilString("GTM and Business Operations Lead"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-01-09T14:59:08-05:00"),
				UpdatedAt:           mustOptDateTime("2026-03-18T10:32:18-04:00"),
				RequisitionID:       NewOptNilString("51"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Location:            NewOptLocation(Location{Name: NewOptNilString("Miami")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5019966007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(5117812007),
				InternalJobID:       NewOptNilInt(4622921007),
				Title:               NewOptNilString("GTM Engineering Intern"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-04-27T17:56:30-04:00"),
				UpdatedAt:           mustOptDateTime("2026-04-27T17:56:30-04:00"),
				RequisitionID:       NewOptNilString("59"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Education:           NewOptNilJobSummaryEducation(JobSummaryEducationEducationRequired),
				Location:            NewOptLocation(Location{Name: NewOptNilString("Remote")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5117812007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(4677712007),
				InternalJobID:       NewOptNilInt(4398021007),
				Title:               NewOptNilString("Head of Client Success (East Cost candidates only)"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2025-05-15T17:13:34-04:00"),
				UpdatedAt:           mustOptDateTime("2026-03-18T10:32:18-04:00"),
				RequisitionID:       NewOptNilString("48"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Location:            NewOptLocation(Location{Name: NewOptNilString("Miami, FL or Remote")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/4677712007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(5024708007),
				InternalJobID:       NewOptNilInt(4579740007),
				Title:               NewOptNilString("Head of Partnerships"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-03-18T15:23:19-04:00"),
				UpdatedAt:           mustOptDateTime("2026-03-18T15:30:59-04:00"),
				RequisitionID:       NewOptNilString("53"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Education:           NewOptNilJobSummaryEducation(JobSummaryEducationEducationRequired),
				Location:            NewOptLocation(Location{Name: NewOptNilString("Miami, FL")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5024708007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(5020246007),
				InternalJobID:       NewOptNilInt(4577366007),
				Title:               NewOptNilString("MBA Intern (Spring and/or Summer '26)"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-01-13T18:31:34-05:00"),
				UpdatedAt:           mustOptDateTime("2026-03-18T10:32:18-04:00"),
				RequisitionID:       NewOptNilString("52"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Location:            NewOptLocation(Location{Name: NewOptNilString("Remote")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5020246007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(5033427007),
				InternalJobID:       NewOptNilInt(4583748007),
				Title:               NewOptNilString("Opportunity Hire, ex-founder/founding engineer"),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-01-23T17:15:16-05:00"),
				UpdatedAt:           mustOptDateTime("2026-03-18T10:32:18-04:00"),
				RequisitionID:       NewOptNilString("55"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Education:           NewOptNilJobSummaryEducation(JobSummaryEducationEducationRequired),
				Location:            NewOptLocation(Location{Name: NewOptNilString("North America")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5033427007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
			{
				ID:                  NewOptInt(5077155007),
				InternalJobID:       NewOptNilInt(4604233007),
				Title:               NewOptNilString("Sales Development Representative "),
				CompanyName:         NewOptNilString("Safari AI"),
				FirstPublished:      mustOptDateTime("2026-03-12T09:36:40-04:00"),
				UpdatedAt:           mustOptDateTime("2026-04-01T09:05:55-04:00"),
				RequisitionID:       NewOptNilString("58"),
				ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
				Location:            NewOptLocation(Location{Name: NewOptNilString("Miami, Florida, United States")}),
				AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/safariai/jobs/5077155007"),
				Language:            NewOptNilString("en"),
				Metadata:            OptNilJobSummaryMetadataItemArray{Set: true, Null: true},
				DataCompliance: []DataCompliance{
					{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
				},
			},
		},
	}, got)
}

func TestListJobsUnknownBoardToken(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{BoardToken: "doesnotexist"})
	require.NoError(t, err)

	_, ok := res.(*ListJobsNotFound)
	assert.True(t, ok, "want *ListJobsNotFound, got %T", res)
}

func TestGetJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{BoardToken: "anthropic", JobID: 4461450008})
	require.NoError(t, err)

	got, ok := res.(*JobDetail)
	require.True(t, ok, "want *JobDetail, got %T", res)

	assert.Equal(t, &JobDetail{
		ID:                  NewOptInt(4461450008),
		Title:               NewOptNilString("Account Executive, AI Native "),
		CompanyName:         NewOptNilString("Anthropic"),
		FirstPublished:      mustOptDateTime("2024-12-20T13:53:38-05:00"),
		UpdatedAt:           mustOptDateTime("2026-06-08T20:49:32-04:00"),
		ApplicationDeadline: OptNilDateTime{Set: true, Null: true},
		RequisitionID:       NewOptNilString("3356"),
		Location:            NewOptLocation(Location{Name: NewOptNilString("San Francisco, CA | New York City, NY")}),
		Content: NewOptNilString(`&lt;div class=&quot;content-intro&quot;&gt;&lt;h2&gt;&lt;strong&gt;About Anthropic&lt;/strong&gt;&lt;/h2&gt;
&lt;p&gt;Anthropic’s mission is to create reliable, interpretable, and steerable AI systems. We want AI to be safe and beneficial for our users and for society as a whole. Our team is a quickly growing group of committed researchers, engineers, policy experts, and business leaders working together to build beneficial AI systems.&lt;/p&gt;&lt;/div&gt;&lt;div&gt;
&lt;h2&gt;About the role&lt;/h2&gt;
&lt;/div&gt;
&lt;div&gt;As a AI Native Account Executive at Anthropic, you’ll drive adoption of safe, frontier AI by securing strategic deals with some of the fastest growing startups in the world. You’ll leverage your consultative sales expertise to propel revenue growth while becoming a trusted partner to customers, helping them embed and deploy AI while uncovering its full range of capabilities. In collaboration with GTM, product, and marketing teams, you’ll continuously refine our value proposition, sales methodology, and market positioning to ensure differentiated value across the landscape.&lt;/div&gt;
&lt;div&gt;&amp;nbsp;&lt;/div&gt;
&lt;div&gt;The ideal candidate will have a passion for developing new market segments, pinpointing high-potential opportunities, and executing strategies to capture them. By driving deployment of Anthropic&#39;s emerging products, you will help enterprises obtain new capabilities while also advancing the ethical development of AI.&lt;/div&gt;
&lt;h2&gt;Responsibilities:&lt;/h2&gt;
&lt;ul&gt;
&lt;li&gt;Win new business and drive revenue for Anthropic. Find your way to the right people at prospective customers, educate them about LLMs, and help them succeed with Anthropic. You’ll own the full sales cycle, from first outbound to launch&lt;/li&gt;
&lt;li&gt;Design and execute innovative sales strategies to meet and exceed revenue quotas. Analyze market landscapes, trends, and dynamics to translate high-level plans into targeted sales activities, partnerships, and campaigns&lt;/li&gt;
&lt;li&gt;Spearhead market expansion by pinpointing new customer segments and use cases. Collaborate cross-functionally to differentiate our offerings and sustain a competitive edge&lt;/li&gt;
&lt;li&gt;Inform product roadmaps and features by gathering customer feedback and conveying market needs. Provide insights that strengthen our value proposition and enhance the customer experience&lt;/li&gt;
&lt;li&gt;Continuously refine the sales methodology by incorporating learnings into playbooks, templates, and best practices. Identify process improvements that optimize sales productivity and consistency&lt;/li&gt;
&lt;/ul&gt;
&lt;h2&gt;You may be a good fit if you have:&lt;/h2&gt;
&lt;ul&gt;
&lt;li&gt;4+ years of experience prospecting and closing startup leads, driving adoption of emerging technologies with a consultative, solutions-oriented sales approach&lt;/li&gt;
&lt;li&gt;Familiarity working within complex sales cycles, selling to technical stakeholders, and securing strategic deals by understanding multifaceted technical requirements and crafting tailored solutions&lt;/li&gt;
&lt;li&gt;Success as a strategic business advisor, deeply understanding the needs of startup founders and creating innovative solutions that align with their vision and help them succeed&lt;/li&gt;
&lt;li&gt;Exposure to negotiating complex, customized commercial agreements with multiple stakeholders&lt;/li&gt;
&lt;li&gt;Demonstrated history of exceeding quota by effectively qualifying and advancing opportunities in a fast-paced work environment&lt;/li&gt;
&lt;li&gt;Excellent communication skills and the ability to present confidently and build connections across all customer levels, from ICs to C-level executives&lt;/li&gt;
&lt;li&gt;A knack for bringing order to chaos and an enthusiastic “roll up your sleeves&#39;&#39; mentality. You are a true team player&lt;/li&gt;
&lt;li&gt;Analytical approach to understanding customer needs combined with creative follow-up to advance opportunities&lt;/li&gt;
&lt;li&gt;Passion for emerging technologies like AI, with interest in ensuring they are developed safely and responsibly&lt;/li&gt;
&lt;li&gt;A strategic, analytical approach to assessing markets combined with creative, tactical execution to capture opportunities&lt;/li&gt;
&lt;li&gt;A passion for and/or experience with advanced AI systems. You feel strongly about ensuring frontier AI systems are developed safely&lt;/li&gt;
&lt;/ul&gt;
&lt;p&gt;&lt;strong&gt;Deadline to apply:&amp;nbsp;&lt;/strong&gt;None. Applications will be reviewed on a rolling basis.&amp;nbsp;&lt;/p&gt;&lt;div class=&quot;content-pay-transparency&quot;&gt;&lt;div class=&quot;pay-input&quot;&gt;&lt;div class=&quot;description&quot;&gt;&lt;p&gt;The annual compensation range for this role is listed below.&amp;nbsp;&lt;/p&gt;
&lt;p&gt;For sales roles, the range provided is the role’s On Target Earnings (&quot;OTE&quot;) range, meaning that the range includes both the sales commissions/sales bonuses target and annual base salary for the role.&lt;/p&gt;&lt;/div&gt;&lt;div class=&quot;title&quot;&gt;Annual Salary:&lt;/div&gt;&lt;div class=&quot;pay-range&quot;&gt;&lt;span&gt;$222,800&lt;/span&gt;&lt;span class=&quot;divider&quot;&gt;&amp;mdash;&lt;/span&gt;&lt;span&gt;$290,000 USD&lt;/span&gt;&lt;/div&gt;&lt;/div&gt;&lt;/div&gt;&lt;div class=&quot;content-conclusion&quot;&gt;&lt;h2&gt;&lt;strong&gt;Logistics&lt;/strong&gt;&lt;/h2&gt;
&lt;p&gt;&lt;strong&gt;Minimum education: &lt;/strong&gt;Bachelor’s degree or an equivalent combination of education, training, and/or experience&lt;/p&gt;
&lt;p&gt;&lt;strong&gt;Required field of study:&amp;nbsp;&lt;/strong&gt;A field relevant to the role as demonstrated through coursework, training, or professional experience&lt;/p&gt;
&lt;p&gt;&lt;strong&gt;Minimum years of experience: &lt;/strong&gt;Years of experience required will correlate with the internal job level requirements for the position&lt;/p&gt;
&lt;p&gt;&lt;strong&gt;Location-based hybrid policy:&lt;/strong&gt; Currently, we expect all staff to be in one of our offices at least 25% of the time. However, some roles may require more time in our offices.&lt;/p&gt;
&lt;p&gt;&lt;strong data-stringify-type=&quot;bold&quot;&gt;Visa sponsorship:&lt;/strong&gt;&amp;nbsp;We do sponsor visas! However, we aren&#39;t able to successfully sponsor visas for every role and every candidate. But if we make you an offer, we will make every reasonable effort to get you a visa, and we retain an immigration lawyer to help with this.&lt;/p&gt;
&lt;p&gt;&lt;strong&gt;We encourage you to apply even if you do not believe you meet every single qualification.&lt;/strong&gt; Not all strong candidates will meet every single qualification as listed.&amp;nbsp; Research shows that people who identify as being from underrepresented groups are more prone to experiencing imposter syndrome and doubting the strength of their candidacy, so we urge you not to exclude yourself prematurely and to submit an application if you&#39;re interested in this work. We think AI systems like the ones we&#39;re building have enormous social and ethical implications. We think this makes representation even more important, and we strive to include a range of diverse perspectives on our team.&lt;br&gt;&lt;br&gt;&lt;strong data-stringify-type=&quot;bold&quot;&gt;Your safety matters to us.&lt;/strong&gt; To protect yourself from potential scams, remember that Anthropic recruiters only contact you from&amp;nbsp;@anthropic.com&amp;nbsp;email addresses. In some cases, we may partner with vetted recruiting agencies who will identify themselves as working on behalf of Anthropic. Be cautious of emails from other domains. Legitimate Anthropic recruiters will never ask for money, fees, or banking information before your first day. If you&#39;re ever unsure about a communication, don&#39;t click any links—visit&amp;nbsp;&lt;u data-stringify-type=&quot;underline&quot;&gt;&lt;a class=&quot;c-link c-link--underline&quot; href=&quot;http://anthropic.com/careers&quot; target=&quot;_blank&quot; data-stringify-link=&quot;http://anthropic.com/careers&quot; data-sk=&quot;tooltip_parent&quot; data-remove-tab-index=&quot;true&quot;&gt;anthropic.com/careers&lt;/a&gt;&lt;/u&gt;&amp;nbsp;directly for confirmed position openings.&lt;/p&gt;
&lt;h2&gt;&lt;strong&gt;How we&#39;re different&lt;/strong&gt;&lt;/h2&gt;
&lt;p&gt;We believe that the highest-impact AI research will be big science. At Anthropic we work as a single cohesive team on just a few large-scale research efforts. And we value impact — advancing our long-term goals of steerable, trustworthy AI — rather than work on smaller and more specific puzzles. We view AI research as an empirical science, which has as much in common with physics and biology as with traditional efforts in computer science. We&#39;re an extremely collaborative group, and we host frequent research discussions to ensure that we are pursuing the highest-impact work at any given time. As such, we greatly value communication skills.&lt;/p&gt;
&lt;p&gt;The easiest way to understand our research directions is to read our recent research. This research continues many of the directions our team worked on prior to Anthropic, including: GPT-3, Circuit-Based Interpretability, Multimodal Neurons, Scaling Laws, AI &amp;amp; Compute, Concrete Problems in AI Safety, and Learning from Human Preferences.&lt;/p&gt;
&lt;h2&gt;&lt;strong&gt;Come work with us!&lt;/strong&gt;&lt;/h2&gt;
&lt;p&gt;Anthropic is a public benefit corporation headquartered in San Francisco. We offer competitive compensation and benefits, optional equity donation matching, generous vacation and parental leave, flexible working hours, and a lovely office space in which to collaborate with colleagues. &lt;strong data-stringify-type=&quot;bold&quot;&gt;Guidance on Candidates&#39; AI Usage:&lt;/strong&gt;&amp;nbsp;Learn about&amp;nbsp;&lt;a class=&quot;c-link&quot; href=&quot;https://www.anthropic.com/candidate-ai-guidance&quot; target=&quot;_blank&quot; data-stringify-link=&quot;https://www.anthropic.com/candidate-ai-guidance&quot; data-sk=&quot;tooltip_parent&quot;&gt;our policy&lt;/a&gt; for using AI in our application process.&lt;/p&gt;&lt;/div&gt;`),
		AbsoluteURL:         mustOptURI("https://job-boards.greenhouse.io/anthropic/jobs/4461450008"),
		Language:            NewOptNilString("en"),
		InternalJobID:       NewOptNilInt(4147866008),
		IncludeAiDisclaimer: OptNilBool{Set: true, Null: true},
		AiDisclaimer:        OptNilString{Set: true, Null: true},
		AiOptOutRequestURL:  OptNilURI{Set: true, Null: true},
		Metadata:            NewOptNilJobDetailMetadataItemArray([]JobDetailMetadataItem{{}}),
		DataCompliance: []DataCompliance{
			{Type: NewOptString("gdpr"), RequiresConsent: NewOptBool(false), RequiresProcessingConsent: NewOptBool(false), RequiresRetentionConsent: NewOptBool(false), RetentionPeriod: OptNilInt{Set: true, Null: true}, DemographicDataConsentApplies: NewOptBool(false)},
		},
		Departments: []Department{
			{ID: NewOptInt(4002062008), Name: NewOptNilString("Sales"), ParentID: OptNilInt{Set: true, Null: true}, ChildIds: []int{}},
		},
		Offices: []Office{
			{ID: NewOptInt(4001219008), Name: NewOptNilString("New York City, NY"), Location: NewOptNilString("New York, New York, United States"), ParentID: OptNilInt{Set: true, Null: true}, ChildIds: []int{}},
			{ID: NewOptInt(4001218008), Name: NewOptNilString("San Francisco, CA"), Location: NewOptNilString("San Francisco, California, United States"), ParentID: OptNilInt{Set: true, Null: true}, ChildIds: []int{}},
		},
	}, got)
}

// TestGetJobWithQuestionsAndPayTransparency guards the nullable optional
// blocks gated behind questions=true/pay_transparency=true: unlike
// TestGetJob's plain fetch, Compliance and PayInputRanges are now populated
// and DemographicQuestions is explicitly present-but-null (not merely
// absent). Asserts structure rather than re-embedding the full application
// form verbatim, since that form's contents are the employer's to change
// and aren't part of this client's contract.
func TestGetJobWithQuestionsAndPayTransparency(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{
		BoardToken:      "anthropic",
		JobID:           4461450008,
		Questions:       NewOptBool(true),
		PayTransparency: NewOptBool(true),
	})
	require.NoError(t, err)

	got, ok := res.(*JobDetail)
	require.True(t, ok, "want *JobDetail, got %T", res)

	require.Len(t, got.Questions, 17)
	assert.Empty(t, got.LocationQuestions)

	require.True(t, got.DemographicQuestions.Set)
	assert.True(t, got.DemographicQuestions.Null)

	compliance, ok := got.Compliance.Get()
	require.True(t, ok)
	require.Len(t, compliance, 4)
	assert.Equal(t, "eeoc", compliance[0].Type.Value)
	assert.Empty(t, compliance[0].Questions)

	want := []PayInputRange{
		{
			MinCents:     NewOptInt(22280000),
			MaxCents:     NewOptInt(29000000),
			CurrencyType: NewOptString("USD"),
			Title:        NewOptString("Annual Salary:"),
			Blurb: NewOptString(`<p>The annual compensation range for this role is listed below.&nbsp;</p>
<p>For sales roles, the range provided is the role’s On Target Earnings ("OTE") range, meaning that the range includes both the sales commissions/sales bonuses target and annual base salary for the role.</p>`),
		},
	}
	assert.Equal(t, want, got.PayInputRanges)
}

func TestGetJobUnknownJobID(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{BoardToken: "anthropic", JobID: 999999999999})
	require.NoError(t, err)

	_, ok := res.(*GetJobNotFound)
	assert.True(t, ok, "want *GetJobNotFound, got %T", res)
}

func TestListJobsWithContent(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{
		BoardToken: "safariai",
		Content:    NewOptBool(true),
	})
	require.NoError(t, err)

	got, ok := res.(*JobListResponse)
	require.True(t, ok, "want *JobListResponse, got %T", res)
	require.Len(t, got.Jobs, 5)
	for _, j := range got.Jobs {
		assert.NotEmpty(t, j.Content.Value, "job %d should carry content", j.ID.Value)
		assert.NotEmpty(t, j.Departments, "job %d should carry departments", j.ID.Value)
		assert.NotEmpty(t, j.Offices, "job %d should carry offices", j.ID.Value)
	}
}

func mustOptDateTime(s string) OptNilDateTime {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return NewOptNilDateTime(tm)
}

func mustOptURI(s string) OptNilURI {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return NewOptNilURI(*u)
}
