# Proves the query string is a no-op: the ASP.NET Core action takes no
# parameters, so keyword/category/limit probes returned a body
# byte-identical to queryjob_rsp.json (verified 2026-07-21, three
# probes). There is consequently no error shape and no server-side
# not-found — unknown values still get the full HTTP 200 dump — which is
# why filtering lives client-side. No paired _rsp.json on purpose. If
# the count assert ever fails, upstream filtering started working and
# the provider's client-side filtering should be revisited.
GET https://hr.quantatw.com/QuantaRecruit/Home/QueryJob?keyword=software&capoid=2&limit=3

HTTP 200
[Asserts]
jsonpath "$.jobResult" count > 3
