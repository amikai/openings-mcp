# Workday CLI Tenant Lookup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `cmd/workday` take `--tenant <slug>` (e.g. `3m`, `att`) instead of a raw Workday CXS `--base-url`, resolved against a curated, embedded list of 200 confirmed S&P 500 Workday tenants.

**Architecture:** A new `internal/provider/workday/companies.yaml` (embedded via `go:embed`) backs a small lookup API in a new `internal/provider/workday/companies.go`. `cmd/workday/main.go` drops `--base-url` entirely, adds a required `--tenant` flag, and gains a `companies` subcommand that lists all confirmed tenants.

**Tech Stack:** Go 1.26, `github.com/goccy/go-yaml` (new dependency) for parsing the embedded YAML, `github.com/peterbourgon/ff/v4` for CLI flags/subcommands (existing), `github.com/stretchr/testify` for test assertions (existing).

## Global Constraints

- Use `github.com/goccy/go-yaml` for YAML parsing, not `gopkg.in/yaml.v3` — the former is actively maintained, the latter is only an indirect dependency today (via ogen codegen tooling) and isn't used directly anywhere in this repo.
- No env-var configuration — this repo's provider CLIs were deliberately stripped of env-var config (see commit `3336ce8`); all input comes from flags.
- `--base-url` is removed from `cmd/workday` with no compatibility shim — nothing else in the repo references it.
- Source data: `internal/provider/workday/sp500_workday_confirmed.json` (a 201-row S&P 500 Workday-tenant list) lives untracked in a **different** git worktree (`/Users/amikai/Workspace/openings-mcp/internal/provider/workday/sp500_workday_confirmed.json`) and is not touched by this plan — it's only the source the tracked `companies.yaml` below was hand-converted from. One row (`FedEx Freight`, tenant `fedex`, site `FXF_External_Career_Site`) was dropped during conversion because it collides with `FedEx` (same tenant `fedex`, different site `FXO_External`) — keeping both would mean whichever the map happened to keep last silently shadowed the other. `companies.yaml` therefore has 200 entries, not 201.

---

## Task 1: Company directory (`internal/provider/workday` package)

**Files:**
- Create: `internal/provider/workday/companies.yaml`
- Create: `internal/provider/workday/companies.go`
- Create: `internal/provider/workday/companies_test.go`
- Modify: `go.mod`, `go.sum` (new dependency)

**Interfaces:**
- Produces (used by Task 2):
  - `type workday.Company struct { Name, Tenant, Instance, Site string }`
  - `func (c workday.Company) BaseURL() string`
  - `func workday.Companies() []workday.Company` — all entries, sorted by `Name`
  - `func workday.CompanyByTenant(tenant string) (workday.Company, bool)` — case-insensitive exact match on `Tenant`

- [ ] **Step 1: Add the `goccy/go-yaml` dependency**

Run:
```
go get github.com/goccy/go-yaml@v1.19.2
```
Expected: `go.mod` gains a new direct `require github.com/goccy/go-yaml v1.19.2` line; `go.sum` gains matching entries.

- [ ] **Step 2: Create `internal/provider/workday/companies.yaml`**

Create the file with exactly this content:

```yaml
- company: "3M"
  tenant: "3m"
  instance: "wd1"
  site: "Search"
- company: "AES Corporation"
  tenant: "aes"
  instance: "wd1"
  site: "AES_US"
- company: "AT&T"
  tenant: "att"
  instance: "wd1"
  site: "ATTGeneral"
- company: "Abbott Laboratories"
  tenant: "abbott"
  instance: "wd5"
  site: "abbottcareers"
- company: "Accenture"
  tenant: "accenture"
  instance: "wd103"
  site: "AccentureCareers"
- company: "Adobe"
  tenant: "adobe"
  instance: "wd5"
  site: "external_experienced"
- company: "Agilent Technologies"
  tenant: "agilent"
  instance: "wd5"
  site: "Agilent_Careers"
- company: "Air Products"
  tenant: "airproducts"
  instance: "wd5"
  site: "AP0001"
- company: "Albemarle"
  tenant: "albemarle"
  instance: "wd5"
  site: "External"
- company: "Allegion"
  tenant: "allegion"
  instance: "wd5"
  site: "careers"
- company: "Alliant Energy"
  tenant: "alliantenergy"
  instance: "wd1"
  site: "alliant"
- company: "Allstate"
  tenant: "allstate"
  instance: "wd5"
  site: "allstate_careers"
- company: "Amcor"
  tenant: "amcor"
  instance: "wd5"
  site: "Amcor_External_Career_Site"
- company: "Ameren"
  tenant: "ameren"
  instance: "wd1"
  site: "External"
- company: "American Electric Power"
  tenant: "aep"
  instance: "wd1"
  site: "AEPCareerSite"
- company: "American International Group (AIG)"
  tenant: "aig"
  instance: "wd1"
  site: "aig"
- company: "Ameriprise Financial"
  tenant: "ameriprise"
  instance: "wd5"
  site: "Ameriprise"
- company: "Amgen"
  tenant: "amgen"
  instance: "wd1"
  site: "Careers"
- company: "Analog Devices"
  tenant: "analogdevices"
  instance: "wd1"
  site: "External"
- company: "Apollo Global Management"
  tenant: "athene"
  instance: "wd5"
  site: "Apollo_Careers"
- company: "Applied Materials"
  tenant: "amat"
  instance: "wd1"
  site: "External"
- company: "Aptiv"
  tenant: "aptiv"
  instance: "wd5"
  site: "APTIV_CAREERS"
- company: "Arch Capital Group"
  tenant: "archgroup"
  instance: "wd1"
  site: "Careers"
- company: "Ares Management"
  tenant: "aresmgmt"
  instance: "wd1"
  site: "External"
- company: "Assurant"
  tenant: "assurant"
  instance: "wd1"
  site: "Assurant_Careers"
- company: "Atmos Energy"
  tenant: "atmosenergy"
  instance: "wd5"
  site: "External_Career_Site"
- company: "Autodesk"
  tenant: "autodesk"
  instance: "wd1"
  site: "Ext"
- company: "AvalonBay Communities"
  tenant: "avalonbay"
  instance: "wd5"
  site: "AVBExternal"
- company: "Baker Hughes"
  tenant: "bakerhughes"
  instance: "wd5"
  site: "BakerHughes"
- company: "Bank of America"
  tenant: "ghr"
  instance: "wd1"
  site: "Lateral-US"
- company: "Baxter International"
  tenant: "baxter"
  instance: "wd1"
  site: "baxter"
- company: "Becton Dickinson"
  tenant: "bdx"
  instance: "wd1"
  site: "EXTERNAL_CAREER_SITE_USA"
- company: "Bio-Techne"
  tenant: "biotechne"
  instance: "wd5"
  site: "Biotechne"
- company: "Biogen"
  tenant: "biibhr"
  instance: "wd3"
  site: "external"
- company: "BlackRock"
  tenant: "blackrock"
  instance: "wd1"
  site: "BlackRock_Professional"
- company: "Blackstone Inc."
  tenant: "blackstone"
  instance: "wd1"
  site: "Blackstone_Careers"
- company: "Boeing"
  tenant: "boeing"
  instance: "wd1"
  site: "INTERN"
- company: "Booking Holdings"
  tenant: "priceline"
  instance: "wd1"
  site: "BookingHoldings"
- company: "Bristol Myers Squibb"
  tenant: "bristolmyerssquibb"
  instance: "wd5"
  site: "BMS"
- company: "Broadcom"
  tenant: "broadcom"
  instance: "wd1"
  site: "External_Career"
- company: "Broadridge Financial Solutions"
  tenant: "broadridge"
  instance: "wd5"
  site: "Careers"
- company: "Brown & Brown"
  tenant: "bbinsurance"
  instance: "wd1"
  site: "Careers"
- company: "Brown-Forman"
  tenant: "bf"
  instance: "wd5"
  site: "USA_Canada (with additional site International)"
- company: "C.H. Robinson"
  tenant: "chrobinson"
  instance: "wd5"
  site: "CHRobinson"
- company: "CDW Corporation"
  tenant: "cdw"
  instance: "wd5"
  site: "careers"
- company: "CF Industries"
  tenant: "cfindustries"
  instance: "wd1"
  site: "careers"
- company: "CME Group"
  tenant: "cmegroup"
  instance: "wd1"
  site: "cme_careers"
- company: "CVS Health"
  tenant: "cvshealth"
  instance: "wd1"
  site: "CVS_Health_Careers"
- company: "Cadence Design Systems"
  tenant: "cadence"
  instance: "wd1"
  site: "External_Careers"
- company: "Capital One"
  tenant: "capitalone"
  instance: "wd12"
  site: "Capital_One"
- company: "Cardinal Health"
  tenant: "cardinalhealth"
  instance: "wd1"
  site: "EXT"
- company: "Carrier Global"
  tenant: "carrier"
  instance: "wd5"
  site: "jobs"
- company: "Caterpillar Inc."
  tenant: "cat"
  instance: "wd5"
  site: "CaterpillarCareers"
- company: "Cboe Global Markets"
  tenant: "cboe"
  instance: "wd1"
  site: "External_Career_CBOE"
- company: "Cencora"
  tenant: "myhrabc"
  instance: "wd5"
  site: "Global"
- company: "Centene Corporation"
  tenant: "centene"
  instance: "wd5"
  site: "Centene_External"
- company: "Chevron Corporation"
  tenant: "chevron"
  instance: "wd5"
  site: "jobs"
- company: "Chipotle Mexican Grill"
  tenant: "chipotle"
  instance: "wd5"
  site: "ChipotleCareers"
- company: "Church & Dwight"
  tenant: "churchdwight"
  instance: "wd1"
  site: "chdcareers"
- company: "Ciena"
  tenant: "ciena"
  instance: "wd5"
  site: "Careers"
- company: "Cigna"
  tenant: "cigna"
  instance: "wd5"
  site: "cignacareers"
- company: "Cisco"
  tenant: "cisco"
  instance: "wd5"
  site: "Cisco_Careers"
- company: "Citigroup"
  tenant: "citi"
  instance: "wd5"
  site: "2"
- company: "Clorox"
  tenant: "clorox"
  instance: "wd1"
  site: "Clorox"
- company: "CoStar Group"
  tenant: "costar"
  instance: "wd1"
  site: "CoStarCareers"
- company: "Coca-Cola Company (The)"
  tenant: "coke"
  instance: "wd1"
  site: "coca-cola-careers"
- company: "Comcast"
  tenant: "comcast"
  instance: "wd5"
  site: "Comcast_Careers"
- company: "Comfort Systems USA"
  tenant: "comfortsystemsusa"
  instance: "wd1"
  site: "Corpcareers"
- company: "ConocoPhillips"
  tenant: "conocophillips"
  instance: "wd1"
  site: "eQuest"
- company: "Constellation Brands"
  tenant: "cbrands"
  instance: "wd5"
  site: "CBI_External_Careers"
- company: "Copart"
  tenant: "copart"
  instance: "wd12"
  site: "Copart"
- company: "Corpay"
  tenant: "corpay"
  instance: "wd103"
  site: "Ext_001"
- company: "Corteva"
  tenant: "corteva"
  instance: "wd5"
  site: "Corteva"
- company: "CrowdStrike"
  tenant: "crowdstrike"
  instance: "wd5"
  site: "crowdstrikecareers"
- company: "DaVita"
  tenant: "davita"
  instance: "wd1"
  site: "DKC_External"
- company: "Danaher Corporation"
  tenant: "danaher"
  instance: "wd1"
  site: "DanaherJobs"
- company: "Deckers Brands"
  tenant: "deckers"
  instance: "wd5"
  site: "Deckers-Brands"
- company: "Dell Technologies"
  tenant: "dell"
  instance: "wd1"
  site: "External"
- company: "Devon Energy"
  tenant: "devonenergy"
  instance: "wd5"
  site: "Careers"
- company: "Dexcom"
  tenant: "dexcom"
  instance: "wd1"
  site: "Dexcom"
- company: "Diamondback Energy"
  tenant: "diamondbackenergy"
  instance: "wd12"
  site: "DBE"
- company: "Dollar Tree"
  tenant: "dollartree"
  instance: "wd5"
  site: "dollartreeus"
- company: "Dow Inc."
  tenant: "dow"
  instance: "wd1"
  site: "ExternalCareers"
- company: "DuPont"
  tenant: "dupont"
  instance: "wd5"
  site: "Jobs"
- company: "Duke Energy"
  tenant: "dukeenergy"
  instance: "wd1"
  site: "Search"
- company: "EchoStar"
  tenant: "echostar"
  instance: "wd5"
  site: "echostar"
- company: "Ecolab"
  tenant: "ecolab"
  instance: "wd1"
  site: "Ecolab_External"
- company: "Edwards Lifesciences"
  tenant: "edwards"
  instance: "wd5"
  site: "EdwardsCareers"
- company: "Elevance Health"
  tenant: "elevancehealth"
  instance: "wd1"
  site: "ANT"
- company: "Equifax"
  tenant: "equifax"
  instance: "wd5"
  site: "External"
- company: "Equinix"
  tenant: "equinix"
  instance: "wd1"
  site: "equest"
- company: "Essex Property Trust"
  tenant: "essex"
  instance: "wd5"
  site: "essexcareers"
- company: "Everest Group"
  tenant: "everestre"
  instance: "wd5"
  site: "careers"
- company: "Eversource Energy"
  tenant: "eversource"
  instance: "wd1"
  site: "ExternalSite"
- company: "Expedia Group"
  tenant: "expedia"
  instance: "wd108"
  site: "search"
- company: "Extra Space Storage"
  tenant: "extraspace"
  instance: "wd5"
  site: "ESS_External"
- company: "F5, Inc."
  tenant: "ffive"
  instance: "wd5"
  site: "f5jobs"
- company: "FactSet"
  tenant: "factset"
  instance: "wd108"
  site: "FactSetCareers"
- company: "Fair Isaac"
  tenant: "fico"
  instance: "wd1"
  site: "External"
- company: "FedEx"
  tenant: "fedex"
  instance: "wd1"
  site: "FXO_External"
- company: "Fidelity National Information Services"
  tenant: "fis"
  instance: "wd5"
  site: "SearchJobs"
- company: "Fifth Third Bancorp"
  tenant: "fifththird"
  instance: "wd5"
  site: "53careers"
- company: "Fiserv"
  tenant: "fiserv"
  instance: "wd5"
  site: "EXT"
- company: "Flex Ltd."
  tenant: "flextronics"
  instance: "wd1"
  site: "Careers"
- company: "Fox Corporation (Class A)"
  tenant: "fox"
  instance: "wd1"
  site: "Domestic"
- company: "Fox Corporation (Class B)"
  tenant: "fox"
  instance: "wd1"
  site: "Domestic"
- company: "Franklin Resources"
  tenant: "franklintempleton"
  instance: "wd5"
  site: "Primary-External-1"
- company: "GE Aerospace"
  tenant: "geaerospace"
  instance: "wd5"
  site: "GE_ExternalSite"
- company: "GE HealthCare"
  tenant: "gehc"
  instance: "wd5"
  site: "GEHC_ExternalSite"
- company: "GE Vernova"
  tenant: "gevernova"
  instance: "wd5"
  site: "Vernova_ExternalSite"
- company: "Gartner"
  tenant: "gartner"
  instance: "wd5"
  site: "EXT"
- company: "Gen Digital"
  tenant: "gen"
  instance: "wd1"
  site: "careers"
- company: "Generac"
  tenant: "generac"
  instance: "wd5"
  site: "External"
- company: "General Dynamics"
  tenant: "gdit"
  instance: "wd5"
  site: "External_Career_Site"
- company: "General Motors"
  tenant: "generalmotors"
  instance: "wd5"
  site: "Careers_GM"
- company: "Genuine Parts Company"
  tenant: "genpt"
  instance: "wd1"
  site: "Careers"
- company: "Gilead Sciences"
  tenant: "gilead"
  instance: "wd1"
  site: "gileadcareers"
- company: "Global Payments"
  tenant: "tsys"
  instance: "wd1"
  site: "TSYS"
- company: "Globe Life"
  tenant: "globelife"
  instance: "wd5"
  site: "Globe-Life-Careers"
- company: "GoDaddy"
  tenant: "godaddy"
  instance: "wd1"
  site: "GoDaddy_careers_events"
- company: "HCA Healthcare"
  tenant: "hcahealthcare"
  instance: "wd3"
  site: "hcacareers"
- company: "HP Inc."
  tenant: "hp"
  instance: "wd5"
  site: "ExternalCareerSite"
- company: "Hartford (The)"
  tenant: "thehartford"
  instance: "wd5"
  site: "Careers_External"
- company: "Henry Schein"
  tenant: "henryschein"
  instance: "wd1"
  site: "External_Careers"
- company: "Hewlett Packard Enterprise"
  tenant: "hpe"
  instance: "wd5"
  site: "Jobsathpe"
- company: "Home Depot (The)"
  tenant: "homedepot"
  instance: "wd5"
  site: "CareerDepot"
- company: "Host Hotels & Resorts"
  tenant: "hosthotels"
  instance: "wd5"
  site: "HostHotels"
- company: "Humana"
  tenant: "humana"
  instance: "wd5"
  site: "Humana_External_Career_Site"
- company: "Huntington Bancshares"
  tenant: "huntington"
  instance: "wd12"
  site: "HNBcareers"
- company: "IDEX Corporation"
  tenant: "idexcorp"
  instance: "wd5"
  site: "IDEX_Careers"
- company: "IQVIA"
  tenant: "iqvia"
  instance: "wd1"
  site: "IQVIA"
- company: "Idexx Laboratories"
  tenant: "idexx"
  instance: "wd1"
  site: "IDEXX"
- company: "Insulet Corporation"
  tenant: "insulet"
  instance: "wd5"
  site: "insuletcareers"
- company: "Intel"
  tenant: "intel"
  instance: "wd1"
  site: "External"
- company: "International Flavors & Fragrances"
  tenant: "iff"
  instance: "wd5"
  site: "IFF_Careers"
- company: "Invesco"
  tenant: "invesco"
  instance: "wd1"
  site: "IVZ"
- company: "Invitation Homes"
  tenant: "invitationhomes"
  instance: "wd1"
  site: "INVH"
- company: "Iron Mountain"
  tenant: "ironmountain"
  instance: "wd5"
  site: "iron-mountain-jobs"
- company: "J.B. Hunt"
  tenant: "jbhunt"
  instance: "wd501"
  site: "Careers"
- company: "Jabil"
  tenant: "jabil"
  instance: "wd5"
  site: "Jabil_Careers"
- company: "Johnson & Johnson"
  tenant: "jj"
  instance: "wd5"
  site: "JJ"
- company: "Johnson Controls"
  tenant: "jci"
  instance: "wd5"
  site: "JCI"
- company: "KLA Corporation"
  tenant: "kla"
  instance: "wd1"
  site: "Search"
- company: "Kenvue"
  tenant: "kenvue"
  instance: "wd5"
  site: "kenvue"
- company: "KeyCorp"
  tenant: "keybank"
  instance: "wd5"
  site: "External_Career_Site"
- company: "Kimberly-Clark"
  tenant: "kimberlyclark"
  instance: "wd1"
  site: "GLOBAL"
- company: "Kimco Realty"
  tenant: "kimcorealty"
  instance: "wd503"
  site: "KimcoCareers"
- company: "Kraft Heinz"
  tenant: "heinz"
  instance: "wd1"
  site: "KraftHeinz_Careers"
- company: "Labcorp"
  tenant: "labcorp"
  instance: "wd1"
  site: "External"
- company: "Las Vegas Sands"
  tenant: "sands"
  instance: "wd1"
  site: "sands_careers"
- company: "Leidos"
  tenant: "leidos"
  instance: "wd5"
  site: "External"
- company: "Lennar"
  tenant: "lennar"
  instance: "wd1"
  site: "Lennar_Jobs"
- company: "Lilly (Eli)"
  tenant: "lilly"
  instance: "wd5"
  site: "LLY"
- company: "Live Nation Entertainment"
  tenant: "livenation"
  instance: "wd503"
  site: "LNExternalSite"
- company: "Loews Corporation"
  tenant: "loewscorp"
  instance: "wd1"
  site: "loewscorp"
- company: "Lumentum"
  tenant: "lumentum"
  instance: "wd5"
  site: "LITE"
- company: "M&T Bank"
  tenant: "mtb"
  instance: "wd5"
  site: "MTB"
- company: "MGM Resorts"
  tenant: "mgmresorts"
  instance: "wd5"
  site: "MGMCareers"
- company: "Marathon Petroleum"
  tenant: "mpc"
  instance: "wd1"
  site: "MPCCareers"
- company: "Marsh McLennan"
  tenant: "mmc"
  instance: "wd1"
  site: "MMC"
- company: "Marvell Technology"
  tenant: "marvell"
  instance: "wd1"
  site: "MarvellCareers"
- company: "Mastercard"
  tenant: "mastercard"
  instance: "wd1"
  site: "CorporateCareers"
- company: "McKesson Corporation"
  tenant: "mckesson"
  instance: "wd3"
  site: "External_Careers"
- company: "Medtronic"
  tenant: "medtronic"
  instance: "wd1"
  site: "MedtronicCareers"
- company: "Merck & Co."
  tenant: "msd"
  instance: "wd5"
  site: "SearchJobs"
- company: "Microchip Technology"
  tenant: "microchiphr"
  instance: "wd5"
  site: "External"
- company: "Micron Technology"
  tenant: "micron"
  instance: "wd1"
  site: "External"
- company: "Mid-America Apartment Communities"
  tenant: "maa"
  instance: "wd1"
  site: "MAA"
- company: "Moderna"
  tenant: "modernatx"
  instance: "wd1"
  site: "M_tx"
- company: "Mondelez International"
  tenant: "mdlz"
  instance: "wd3"
  site: "External"
- company: "Monolithic Power Systems"
  tenant: "monolithicpower"
  instance: "wd12"
  site: "MPS_Careers"
- company: "Morgan Stanley"
  tenant: "ms"
  instance: "wd5"
  site: "External"
- company: "Mosaic Company (The)"
  tenant: "mosaic"
  instance: "wd5"
  site: "mosaic"
- company: "Motorola Solutions"
  tenant: "motorolasolutions"
  instance: "wd5"
  site: "Careers"
- company: "NXP Semiconductors"
  tenant: "nxp"
  instance: "wd3"
  site: "careers"
- company: "Nasdaq, Inc."
  tenant: "nasdaq"
  instance: "wd1"
  site: "Global_External_Site"
- company: "Netflix"
  tenant: "netflix"
  instance: "wd108"
  site: "Netflix"
- company: "News Corp (Class A)"
  tenant: "dowjones"
  instance: "wd1"
  site: "News_Corp_Careers"
- company: "News Corp (Class B)"
  tenant: "dowjones"
  instance: "wd1"
  site: "News_Corp_Careers"
- company: "NiSource"
  tenant: "nisource"
  instance: "wd1"
  site: "NiSource"
- company: "Nike, Inc."
  tenant: "nike"
  instance: "wd1"
  site: "nke"
- company: "Nordson Corporation"
  tenant: "nordsonhcm"
  instance: "wd5"
  site: "nordsoncareers"
- company: "Northern Trust"
  tenant: "ntrs"
  instance: "wd1"
  site: "northerntrust"
- company: "Northrop Grumman"
  tenant: "ngc"
  instance: "wd1"
  site: "Northrop_Grumman_External_Site"
- company: "Norwegian Cruise Line Holdings"
  tenant: "nclh"
  instance: "wd108"
  site: "NCLH_Careers"
- company: "Nvidia"
  tenant: "nvidia"
  instance: "wd5"
  site: "NVIDIAExternalCareerSite"
- company: "O'Reilly Automotive"
  tenant: "oreillyauto"
  instance: "wd1"
  site: "oreilly"
- company: "Occidental Petroleum"
  tenant: "oxy"
  instance: "wd5"
  site: "Corporate"
- company: "Old Dominion"
  tenant: "odfl"
  instance: "wd1"
  site: "ODFL_Careers"
- company: "Oneok"
  tenant: "oneok"
  instance: "wd1"
  site: "ONEOK"
- company: "Otis Worldwide"
  tenant: "otis"
  instance: "wd5"
  site: "REC_Ext_Gateway"
- company: "PNC Financial Services"
  tenant: "pnc"
  instance: "wd5"
  site: "External"
- company: "PayPal"
  tenant: "paypal"
  instance: "wd1"
  site: "jobs"
- company: "Pentair"
  tenant: "pentair"
  instance: "wd5"
  site: "Pentair_Careers"
- company: "Pfizer"
  tenant: "pfizer"
  instance: "wd1"
  site: "PfizerCareers"
- company: "Procter & Gamble"
  tenant: "pg"
  instance: "wd5"
  site: "1000"
- company: "Prologis"
  tenant: "prologis"
  instance: "wd5"
  site: "Prologis_External_Careers"
- company: "Visa Inc."
  tenant: "visa"
  instance: "wd5"
  site: "Visa"
- company: "Workday, Inc."
  tenant: "workday"
  instance: "wd5"
  site: "Workday"
- company: "eBay Inc."
  tenant: "ebay"
  instance: "wd5"
  site: "TCGPlayer_External_Career"
```

- [ ] **Step 3: Write the failing tests**

Create `internal/provider/workday/companies_test.go`:

```go
package workday

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompanyByTenant(t *testing.T) {
	tests := []struct {
		name   string
		tenant string
		want   Company
		wantOk bool
	}{
		{
			name:   "exact lowercase match",
			tenant: "3m",
			want:   Company{Name: "3M", Tenant: "3m", Instance: "wd1", Site: "Search"},
			wantOk: true,
		},
		{
			name:   "case-insensitive match",
			tenant: "3M",
			want:   Company{Name: "3M", Tenant: "3m", Instance: "wd1", Site: "Search"},
			wantOk: true,
		},
		{
			name:   "another known tenant",
			tenant: "att",
			want:   Company{Name: "AT&T", Tenant: "att", Instance: "wd1", Site: "ATTGeneral"},
			wantOk: true,
		},
		{
			name:   "unknown tenant",
			tenant: "doesnotexist-tenant-xyz",
			want:   Company{},
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CompanyByTenant(tt.tenant)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompanyBaseURL(t *testing.T) {
	c := Company{Tenant: "3m", Instance: "wd1", Site: "Search"}
	assert.Equal(t, "https://3m.wd1.myworkdayjobs.com/wday/cxs/3m/Search", c.BaseURL())
}

func TestCompaniesSortedAndComplete(t *testing.T) {
	cs := Companies()
	assert.Len(t, cs, 200)
	assert.True(t, sort.SliceIsSorted(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name }))
}
```

- [ ] **Step 4: Run the tests and confirm they fail to compile**

Run: `go test ./internal/provider/workday/... -run TestCompan -v`
Expected: FAIL — build error, e.g. `undefined: Company` (companies.go doesn't exist yet).

- [ ] **Step 5: Implement `internal/provider/workday/companies.go`**

```go
package workday

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed Workday CXS tenant for a public company, drawn from
// a curated S&P 500 list (internal/provider/workday/companies.yaml). It's
// keyed by tenant slug (e.g. "3m", "att") rather than display name — tenant
// slugs are unique*, lowercase, and punctuation-free, unlike display names
// such as "AT&T" or "Workday, Inc.".
//
// *Two harmless exceptions share a tenant across two rows with an identical
// instance/site (Fox Corporation's two share classes under "fox", News
// Corp's two share classes under "dowjones") — both rows resolve to the same
// BaseURL either way, so which one a lookup returns doesn't matter.
type Company struct {
	Name     string `yaml:"company" json:"company"`
	Tenant   string `yaml:"tenant" json:"tenant"`
	Instance string `yaml:"instance" json:"instance"`
	Site     string `yaml:"site" json:"site"`
}

// BaseURL builds this company's Workday CXS base URL, e.g.
// https://3m.wd1.myworkdayjobs.com/wday/cxs/3m/Search — the same
// {tenant}.{instance}.myworkdayjobs.com/wday/cxs/{tenant}/{site} shape
// documented on PublicSiteURL in path.go.
func (c Company) BaseURL() string {
	return fmt.Sprintf("https://%s.%s.myworkdayjobs.com/wday/cxs/%s/%s", c.Tenant, c.Instance, c.Tenant, c.Site)
}

var (
	companies         = mustLoadCompanies()
	companiesByTenant = buildTenantIndex(companies)
)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("workday: parse companies.yaml: %v", err))
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	return cs
}

func buildTenantIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Tenant)] = c
	}
	return m
}

// Companies returns every confirmed Workday tenant, sorted by company name.
func Companies() []Company {
	return companies
}

// CompanyByTenant looks up a confirmed tenant by slug, case-insensitively.
func CompanyByTenant(tenant string) (Company, bool) {
	c, ok := companiesByTenant[strings.ToLower(tenant)]
	return c, ok
}
```

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `go test ./internal/provider/workday/... -run TestCompan -v`
Expected: PASS — `TestCompanyByTenant`, `TestCompanyBaseURL`, `TestCompaniesSortedAndComplete` all pass.

- [ ] **Step 7: Run the full package test suite to confirm nothing else broke**

Run: `go test ./internal/provider/workday/...`
Expected: PASS (existing `path_test.go` and `client_test.go` tests still pass unchanged).

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/provider/workday/companies.yaml internal/provider/workday/companies.go internal/provider/workday/companies_test.go
git commit -m "feat(workday): add embedded S&P 500 tenant directory"
```

---

## Task 2: `cmd/workday` tenant flag and `companies` subcommand

**Files:**
- Modify: `cmd/workday/main.go` (full rewrite of flag setup, `runFacets`, `runSearch`; adds `runCompanies`)
- Create: `cmd/workday/main_test.go`

**Interfaces:**
- Consumes: `workday.CompanyByTenant(tenant string) (workday.Company, bool)`, `workday.Companies() []workday.Company`, `workday.Company.BaseURL() string` (all from Task 1)
- Produces: `runFacets(ctx context.Context, tenant string, timeout time.Duration, searchText string, facetArgs []string, format string) error`, `runSearch(ctx context.Context, tenant string, timeout time.Duration, searchText string, limit, offset int, facetArgs []string, format string) error`, `runCompanies(format string) error` — same names as today, but the second parameter is now a tenant slug instead of a raw base URL

- [ ] **Step 1: Write the failing tests**

Create `cmd/workday/main_test.go`:

```go
package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunFacetsMissingTenant(t *testing.T) {
	err := runFacets(context.Background(), "", time.Second, "", nil, "text")
	assert.ErrorContains(t, err, "--tenant is required")
}

func TestRunFacetsUnknownTenant(t *testing.T) {
	err := runFacets(context.Background(), "doesnotexist-tenant-xyz", time.Second, "", nil, "text")
	assert.ErrorContains(t, err, `tenant "doesnotexist-tenant-xyz" not found`)
	assert.ErrorContains(t, err, "workday companies")
}

func TestRunSearchMissingTenant(t *testing.T) {
	err := runSearch(context.Background(), "", time.Second, "", 20, 0, nil, "text")
	assert.ErrorContains(t, err, "--tenant is required")
}

func TestRunSearchUnknownTenant(t *testing.T) {
	err := runSearch(context.Background(), "doesnotexist-tenant-xyz", time.Second, "", 20, 0, nil, "text")
	assert.ErrorContains(t, err, `tenant "doesnotexist-tenant-xyz" not found`)
	assert.ErrorContains(t, err, "workday companies")
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test ./cmd/workday/... -run TestRun -v`
Expected: FAIL. The current `runFacets`/`runSearch` treat their second argument as a raw base URL (not a tenant slug) and don't validate it against `workday.CompanyByTenant`, so `TestRunFacetsMissingTenant`/`TestRunSearchMissingTenant` see no `"--tenant is required"` error, and the unknown-tenant tests see a client/network error instead of the `"tenant ... not found"` message.

- [ ] **Step 3: Rewrite `cmd/workday/main.go`**

Replace the entire file with:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	workday "github.com/amikai/openings-mcp/internal/provider/workday"
)

func main() {
	rootFlags := ff.NewFlagSet("workday")
	var (
		tenant  = rootFlags.StringLong("tenant", "", "confirmed Workday tenant slug, e.g. 3m, att (see 'workday companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "workday",
		Usage: "workday --tenant TENANT [FLAGS] <companies|facets|search> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "workday companies [--format text|json]",
		ShortHelp: "list confirmed Workday tenants (company name and tenant slug)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	facetsFlags := ff.NewFlagSet("facets").SetParent(rootFlags)
	var (
		facetsSearchText = facetsFlags.StringLong("search-text", "", "free-text keyword search to narrow the facet tree")
		facetsFacetArgs  = facetsFlags.StringListLong("facet", "facet filter as name=id, repeatable")
	)
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "workday --tenant TENANT facets [--search-text TEXT] [--facet name=id ...] [--format text|json]",
		ShortHelp: "discover a tenant's current facet tree (categories, locations, ...)",
		Flags:     facetsFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runFacets(ctx, *tenant, *timeout, *facetsSearchText, *facetsFacetArgs, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		searchText      = searchFlags.StringLong("search-text", "", "free-text keyword search")
		limit           = searchFlags.IntLong("limit", 20, "page size")
		offset          = searchFlags.IntLong("offset", 0, "zero-based result offset")
		searchFacetArgs = searchFlags.StringListLong("facet", "facet filter as name=id, repeatable")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "workday --tenant TENANT search [--search-text TEXT] [--limit N] [--offset N] [--facet name=id ...] [--format text|json]",
		ShortHelp: "search jobs and fetch full detail for each result",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *tenant, *timeout, *searchText, *limit, *offset, *searchFacetArgs, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, facets, or search) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// parseFacets turns repeated "--facet name=id" flag values into an
// AppliedFacets map. Repeating the same name appends to that facet's id
// list (OR'd within a facet); different names key different facets (AND'd
// together) — matches AppliedFacets's map[string][]string shape 1:1.
func parseFacets(raw []string) (workday.AppliedFacets, error) {
	af := workday.AppliedFacets{}
	for _, f := range raw {
		name, id, ok := strings.Cut(f, "=")
		if !ok || name == "" || id == "" {
			return nil, fmt.Errorf("invalid --facet %q, want name=id", f)
		}
		af[name] = append(af[name], id)
	}
	return af, nil
}

// runCompanies lists every confirmed Workday tenant embedded in the CLI
// (internal/provider/workday/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := workday.Companies()

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Tenant)
	}
	return nil
}

// runFacets discovers a tenant's current facet tree via a search whose only
// job is to read back JobsResponse.Facets — Limit is 1 because the actual
// jobPostings aren't used here (see openapi.yaml's note that every /jobs
// response, filtered or not, carries the full current facet tree).
func runFacets(ctx context.Context, tenant string, timeout time.Duration, searchText string, facetArgs []string, format string) error {
	if tenant == "" {
		return fmt.Errorf("--tenant is required")
	}
	company, ok := workday.CompanyByTenant(tenant)
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", tenant)
	}

	appliedFacets, err := parseFacets(facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := workday.NewClient(company.BaseURL())
	if err != nil {
		return err
	}

	search, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         1,
		Offset:        0,
		SearchText:    searchText,
	})
	if err != nil {
		return err
	}

	// Get returns a nil slice when the tenant omitted facets or sent null.
	facets, _ := search.Facets.Get()

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(facets)
	}

	for _, node := range facets {
		printFacetNode(node, 0)
	}
	return nil
}

// printFacetNode recursively prints one facet tree node. A node with a
// facetParameter is a group (printed as "facetParameter (descriptor)",
// descriptor omitted when unset — some top-level groups like
// locationMainGroup have none); a node without one is a leaf, printed as
// "descriptor  id=...  count=...". Grouping keys on facetParameter rather
// than len(Values) so a group whose Values are momentarily empty isn't
// mis-rendered as a leaf.
func printFacetNode(node workday.FacetNode, depth int) {
	indent := strings.Repeat("  ", depth)
	if node.FacetParameter.Set {
		label := node.FacetParameter.Value
		if node.Descriptor.Set {
			label = fmt.Sprintf("%s (%s)", label, node.Descriptor.Value)
		}
		fmt.Println(indent + label)
		for _, child := range node.Values {
			printFacetNode(child, depth+1)
		}
		return
	}
	fmt.Printf("%s%s  id=%s  count=%d\n", indent, node.Descriptor.Value, node.ID.Value, node.Count.Value)
}

// jobResultJSON is the --format json shape for one search result: the
// search summary merged with its fetched detail (or, if the detail fetch
// failed, a fallback link plus Error instead of Description).
type jobResultJSON struct {
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Location    string   `json:"location,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	PostedOn    string   `json:"postedOn,omitempty"`
	Description string   `json:"description,omitempty"`
	JobReqId    string   `json:"jobReqId,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type searchResultJSON struct {
	Total int             `json:"total"`
	Jobs  []jobResultJSON `json:"jobs"`
}

// runSearch searches jobs, then fetches full detail for every result
// (mirrors cmd/nvidia's report behavior: a posting with no ExternalPath is
// listed with a "no detail available" note rather than silently dropped, so
// "showing N" always matches the page's posting count) — one page per
// invocation, no auto-pagination.
func runSearch(ctx context.Context, tenant string, timeout time.Duration, searchText string, limit, offset int, facetArgs []string, format string) error {
	if tenant == "" {
		return fmt.Errorf("--tenant is required")
	}
	company, ok := workday.CompanyByTenant(tenant)
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", tenant)
	}

	appliedFacets, err := parseFacets(facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	baseURL := company.BaseURL()
	client, err := workday.NewClient(baseURL)
	if err != nil {
		return err
	}

	search, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         limit,
		Offset:        offset,
		SearchText:    searchText,
	})
	if err != nil {
		return err
	}

	results := make([]jobResultJSON, 0, len(search.JobPostings))
	for _, job := range search.JobPostings {
		results = append(results, fetchJobResult(ctx, client, baseURL, job))
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: search.Total, Jobs: results})
	}

	fmt.Printf("Workday Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total, len(results))
	for i, r := range results {
		fmt.Printf("%d. %s\n", i+1, r.Title)
		if r.PostedOn != "" {
			fmt.Printf("Posted: %s\n", r.PostedOn)
		}
		if r.URL != "" {
			fmt.Printf("URL: %s\n", r.URL)
		}
		printResultLocations(r)
		switch {
		case r.Error != "":
			fmt.Printf("(job detail unavailable: %s)\n", r.Error)
		case r.Description != "":
			fmt.Printf("Description:\n%s\n", r.Description)
		}
		fmt.Println()
	}
	return nil
}

func printResultLocations(r jobResultJSON) {
	if len(r.Locations) > 0 {
		fmt.Println("Locations:")
		for _, l := range r.Locations {
			fmt.Printf("  - %s\n", l)
		}
		return
	}
	if r.Location != "" {
		fmt.Printf("Location: %s\n", r.Location)
	}
}

// fetchJobResult fetches full detail for one job summary. A detail-fetch
// failure is non-fatal: it falls back to a derived public site URL and the
// summary's aggregate LocationsText, and records the error instead of a
// description, so one bad job doesn't abort the whole search — mirrors
// cmd/nvidia's existing per-job fallback behavior. A summary with no
// ExternalPath (an incomplete/transient Workday posting) can't be fetched at
// all, so it's returned with a "no detail available" note rather than dropped.
func fetchJobResult(ctx context.Context, client *workday.Client, baseURL string, job workday.JobSummary) jobResultJSON {
	r := jobResultJSON{Title: job.Title.Value, PostedOn: job.PostedOn.Value}

	if job.ExternalPath.Value == "" {
		r.Error = "listing has no externalPath"
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	location, titleSlug, ok := workday.SplitExternalPath(job.ExternalPath.Value)
	if !ok {
		r.Error = fmt.Sprintf("could not split externalPath %q", job.ExternalPath.Value)
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	detail, err := client.GetJobDetail(ctx, workday.GetJobDetailParams{Location: location, TitleSlug: titleSlug})
	if err != nil {
		r.Error = err.Error()
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	info := detail.JobPostingInfo
	// Overwrite the summary's title/postedOn only when the detail actually
	// carries a value — a detail response that omits postedOn (optional) or
	// returns an empty title must not blank out the good summary value.
	if info.Title != "" {
		r.Title = info.Title
	}
	if info.PostedOn.Set {
		r.PostedOn = info.PostedOn.Value
	}
	r.JobReqId = info.JobReqId.Value
	if info.ExternalUrl.Set {
		r.URL = info.ExternalUrl.Value
	} else {
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
	}

	itemized := make([]string, 0, 1+len(info.AdditionalLocations))
	if info.Location.Set {
		itemized = append(itemized, info.Location.Value)
	}
	itemized = append(itemized, info.AdditionalLocations...)
	setLocations(&r, itemized...)

	description, err := html2text.FromString(info.JobDescription, html2text.Options{})
	if err != nil {
		description = info.JobDescription
	}
	r.Description = description

	return r
}

// setLocations fills both the singular Location (first entry, for quick
// access) and the full Locations array (only when there's more than one, to
// avoid a redundant one-element array alongside the singular field) —
// mirrors cmd/nvidia's printLocations singular/plural distinction.
func setLocations(r *jobResultJSON, locations ...string) {
	if len(locations) == 0 {
		return
	}
	r.Location = locations[0]
	if len(locations) > 1 {
		r.Locations = locations
	}
}

// fallbackURL builds a best-effort public job link when the detail fetch
// (which carries the authoritative externalUrl) fails. Falls back to
// externalPath alone if the base URL can't be resolved to a public site
// origin either.
func fallbackURL(baseURL, externalPath string) string {
	site, err := workday.PublicSiteURL(baseURL)
	if err != nil {
		return externalPath
	}
	// externalPath usually starts with "/", but SplitExternalPath treats a
	// missing leading slash as just another malformed shape that lands here —
	// don't let it glue the site segment and location together.
	if !strings.HasPrefix(externalPath, "/") {
		externalPath = "/" + externalPath
	}
	return site + externalPath
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./cmd/workday/... -run TestRun -v`
Expected: PASS — all four tests pass.

- [ ] **Step 5: Build the CLI and manually confirm the `companies` subcommand works**

Run:
```
go build -o /tmp/workday-cli ./cmd/workday && /tmp/workday-cli companies | head -5
```
Expected: prints 5 lines in the form `Company Name (tenant)`, e.g. starting with `3M (3m)`.

Run:
```
/tmp/workday-cli --tenant doesnotexist search
```
Expected: exits non-zero, prints `err: tenant "doesnotexist" not found; run 'workday companies' to see supported tenants` to stderr.

- [ ] **Step 6: Run the full repo test suite**

Run: `go test ./...`
Expected: PASS (no regressions in other packages).

- [ ] **Step 7: Commit**

```bash
git add cmd/workday/main.go cmd/workday/main_test.go
git commit -m "feat(workday): replace --base-url with --tenant lookup and add companies subcommand"
```
