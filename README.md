<h1 align="center">openings-mcp</h1>

<p align="center">
  <strong>One MCP server to search job boards and company career sites.</strong>
</p>

**openings-mcp** searches job listings across job boards and company career
sites from any MCP client: Claude Code, Codex, Gemini CLI, and others.

- **Job boards**: **[104](https://www.104.com.tw)**, **[Cake](https://www.cake.me)**,
  **[Jobindex](https://www.jobindex.dk)** (Denmark),
  **[マイナビ転職](https://tenshoku.mynavi.jp)** (Japan),
  **[LinkedIn](https://www.linkedin.com)**, and **[Indeed](https://www.indeed.com)**
  (public search).
- **Company career sites**: 3,500+ companies hosted on the
  **[Workday](https://www.workday.com)**, **[Ashby](https://www.ashbyhq.com)**,
  **[Greenhouse](https://www.greenhouse.com)**, **[Lever](https://www.lever.co)**,
  **[Teamtailor](https://www.teamtailor.com)**, **[Recruitee](https://recruitee.com)**,
  **[Eightfold](https://eightfold.ai)**,
  **[SAP SuccessFactors](https://www.sap.com/products/hcm/recruiting-software.html)**,
  **[SmartRecruiters](https://www.smartrecruiters.com)**,
  **[Workable](https://www.workable.com)**,
  **[Rippling](https://www.rippling.com/recruiting)**,
  **[iCIMS](https://www.icims.com)**,
  **[Oracle Recruiting Cloud](https://www.oracle.com/human-capital-management/recruiting/)**,
  **[JOIN](https://join.com)**,
  and **[UKG Pro (UltiPro)](https://www.ukg.com/products/ukg-pro)**
  ATS platforms, all behind one company-search tool. A company outside the
  built-in roster works too: pass its careers-page URL on any of those platforms
  except Eightfold, SAP SuccessFactors, and JOIN, which support roster companies only.
- **Dedicated sites**: **[Google Careers](https://www.google.com/about/careers/applications/jobs)**,
  **[NVIDIA careers](https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite)**, and
  **[TSMC careers](https://careers.tsmc.com)**.

⚠️ Token use adds up fast when your client explores multiple postings or
fetches full job details, so consider a cheaper model.

<p align="center">
  <img src="assets/openings_demo.gif" alt="Demo of openings-mcp searching job listings from Claude Code" width="800">
</p>

> **Disclaimer:** This is an unofficial, personal-use tool. It is not affiliated
> with, endorsed by, or sponsored by 104 Corporation, TSMC, or any other job
> board or company whose listings it searches. It calls each site's public web
> endpoints; please respect their terms of service and use it at a reasonable,
> low frequency. The project ships no scraped job data.

## Install

With Homebrew:

```
brew install --cask amikai/tap/openings-mcp
```

With Go:

```
go install github.com/amikai/openings-mcp/cmd/openings-mcp@latest
```

Upgrade by rerunning the same command; pin a version with `@vX.Y.Z`.

Without Go: download the archive for your platform from
[Releases](https://github.com/amikai/openings-mcp/releases) and put
`openings-mcp` on your PATH.

With Docker (multi-arch: linux/amd64, linux/arm64):

```
docker pull ghcr.io/amikai/openings-mcp
```

## Add the MCP server to your tool

With `openings-mcp` on your PATH:

**Claude Code**

```
claude mcp add openings-mcp -- openings-mcp
```

**Codex**

```
codex mcp add openings-mcp -- openings-mcp
```

**Gemini CLI**

```
gemini mcp add openings-mcp openings-mcp
```

With Docker instead, replace `openings-mcp` with `docker run -i --rm ghcr.io/amikai/openings-mcp`, e.g.:

```
claude mcp add openings-mcp -- docker run -i --rm ghcr.io/amikai/openings-mcp
```

## Disclaimer

This is an unofficial tool. It is not affiliated with, endorsed by, or
supported by 104 Corporation, TSMC, or any other job board or company whose
listings it searches.

Some providers rely on undocumented APIs that may change or stop working at
any time without notice. Job listing data belongs to the respective sites. The
server fetches it on your behalf when you invoke a tool and stores or
redistributes nothing.

Use this tool for personal job searching at a human pace. Every request goes
out from your own machine, and a site may throttle or temporarily block your
IP if too many arrive too quickly. The server does not rate-limit requests,
so a client that pages through many results or fetches lots of job details in
one go can trip these limits. The server never logs in, so your accounts on
these sites are not at risk. You are responsible for complying with the terms of
service of each site you query; do not use it for bulk scraping or data
harvesting.

## Credits

Inspired by these MCP job-search servers:

- [job104-mcp](https://github.com/mozzan/job104-mcp)
- [jobspy-mcp-server](https://github.com/borgius/jobspy-mcp-server)
- [mcp-jobs](https://github.com/mergedao/mcp-jobs)
- [104-mcp-server](https://github.com/bigbrainw/104-mcp-server)

Company roster data cross-referenced from:

- [OpenPostings](https://github.com/Masterjx9/OpenPostings) — company/ATS discovery data used to expand the curated roster

Jobindex search reference:

- [ai-job-search](https://github.com/MadsLorentzen/ai-job-search)

## License

[MIT](LICENSE)
