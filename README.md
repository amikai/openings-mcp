<h1 align="center">openings-mcp</h1>

<p align="center">
  <strong>Search job listings from any MCP client — job boards and company career sites, one server.</strong>
</p>

**openings-mcp** searches job listings across job boards and company career sites —
currently **[104](https://www.104.com.tw)**, **[Cake](https://www.cake.me)**,
**[Google Careers](https://www.google.com/about/careers/applications/jobs)**,
**[NVIDIA careers](https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite)**, and
**[TSMC careers](https://careers.tsmc.com)** —
from any MCP client: Claude Code, Codex, Gemini CLI, and others.

⚠️ This server can consume a lot of tokens, especially when your client explores
multiple postings or fetches full job details, so use `Gemini Flash` or
`Claude Haiku`, and keep `effort` low.

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
any time without notice. Job listing data belongs to the respective sites and
is fetched on your behalf when you invoke a tool — nothing is stored or
redistributed by this project.

Use this tool for personal job searching at a human pace. You are responsible
for complying with the terms of service of each site you query; do not use it
for bulk scraping or data harvesting.

## License

[MIT](LICENSE)
