# Teamtailor Provider Package — Design

## Scope

Build the handwritten provider layer around the generated Teamtailor Career
Site Feed client: fixtures, reusable mock server, and a small curated roster.
The user-facing ATS adapter is a later stage.

## Roster model

One Teamtailor tenant is addressed by its career-site hostname, so the roster
schema is:

```yaml
- company: "bunny.net"
  host: "bunnynet.teamtailor.com"
```

`Company.Host` is lowercase and doubles as the unified ATS slug. `CareersURL`
returns `https://<host>/jobs`. The package embeds `companies.yaml`, sorts
`Companies` by display name, and exposes `CompaniesByHost` for case-insensitive
lookup. Tests reject duplicate hosts.

The five-entry seed roster proves both regional host layouts and was live
verified on 2026-07-13: each `/jobs.json` returned 200, at least one item, and
a feed title matching the roster name.

| Company | Host | Jobs at verification |
| --- | --- | ---: |
| bunny.net | `bunnynet.teamtailor.com` | 8 |
| Knauf Belgium | `knaufsemea.teamtailor.com` | 3 |
| Teamtailor | `career.teamtailor.com` | 14 |
| Tiptapp | `tiptapp.teamtailor.com` | 1 |
| Village Automotive Group | `villageautomotivegroup.na.teamtailor.com` | 13 |

## Fixtures and mock server

`jobs_rsp.json` is an untouched live feed. `NewMockServer` serves it from
`/jobs.json` with the real `application/feed+json` media type and returns an
empty 404 for other paths. The generated-client test asserts representative
identity, timestamp, HTML, organization, and nullable address behavior.

## Constraints

- Captured JSON is never hand-authored or normalized.
- Generated `oas_*_gen.go` files are never hand-edited.
- Custom-domain tenants can be represented by the same `host` field without a
  schema change.
