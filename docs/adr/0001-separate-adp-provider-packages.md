# Separate provider packages for ADP MyJobs and ADP Workforce Now

ADP runs two unrelated public career-board surfaces — MyJobs
(`myjobs.adp.com`) and Workforce Now (`workforcenow.adp.com`) — and we serve
each from its own provider package and its own `ats.Adapter`
(`adp_myjobs`, `adp_wfn`) rather than one merged `adp` provider. They share a
vendor and nothing else: MyJobs is keyed by a readable tenant slug and
Workforce Now by a `cid` GUID; MyJobs filters through positional `FIELD1`..`FIELD5`
slot codes on an OData-style `$filter`, Workforce Now through HTTP headers
with a pair-based grammar; their pagination, locale handling, and failure
modes have no overlap. A merged package would be two code paths in every
method with a shared name as the only benefit.

## Considered options

**One `adp` package routing by roster entry.** Rejected. The only thing it
buys is that a user typing "ADP" reaches both boards — and the registry
already resolves a company by name or careers URL, so nobody types the
provider name anyway. The cost is permanent: every method branches on which
surface the entry belongs to, and `adp_myjobs`'s hard-won filter quirks get
entangled with an unrelated API's.

## Consequences

A company hosting jobs on both surfaces appears in both rosters. That is
fine: cross-adapter name collisions are expected, `Registry.Resolve`
disambiguates them by careers URL, and they only need naming in the
`TestCompanyCollisionReport` golden file. Within a single roster, collisions
remain a curation bug that fails `ats.NewRegistry` at startup by design.
