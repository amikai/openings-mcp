package ats

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/maypok86/otter/v2"
)

// DumpCacheConfig configures a process-local full-dump cache.
// Callers that want no caching pass nil to adapters instead of constructing one.
type DumpCacheConfig struct {
	// TTL is how long each board dump is retained after write.
	// Values <= 0 become DefaultDumpCacheTTL in NewDumpCache.
	TTL time.Duration
	// MaxEntries is otter's MaximumSize (number of provider+slug boards).
	// Values <= 0 become DefaultDumpCacheMaxEntries.
	MaxEntries int
}

const (
	// DefaultDumpCacheTTL is the default write-expiry for board dumps.
	DefaultDumpCacheTTL = 10 * time.Minute
	// DefaultDumpCacheMaxEntries is the default max number of cached boards.
	DefaultDumpCacheMaxEntries = 100
)

// dumpCacheEntry is one stored board dump for a single provider+slug key.
type dumpCacheEntry struct {
	// jobs is owned by the cache after Set. Callers receive this slice as
	// read-only (see dumpJob immutability contract); they must cloneDumpJobs
	// before any mutation.
	jobs []dumpJob
	// side is optional adapter metadata (e.g. *herp.CompanyBoard), shared read-only.
	side any
}

// DumpCache is a short-TTL, size-bounded cache of full-board dumps for a
// single-user stdio MCP session. A nil *DumpCache means no caching
// (getOrLoadDump passes through to load). Cached dumps are shared read-only;
// callers must not mutate returned jobs or side.
type DumpCache struct {
	cfg   DumpCacheConfig
	cache *otter.Cache[string, *dumpCacheEntry]
}

// NewDumpCache builds an enabled cache. Callers that want no caching pass
// nil to adapters instead of constructing one.
func NewDumpCache(cfg DumpCacheConfig) *DumpCache {
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultDumpCacheTTL
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = DefaultDumpCacheMaxEntries
	}
	return &DumpCache{
		cfg: cfg,
		cache: otter.Must(&otter.Options[string, *dumpCacheEntry]{
			MaximumSize:      cfg.MaxEntries,
			ExpiryCalculator: otter.ExpiryWriting[string, *dumpCacheEntry](cfg.TTL),
		}),
	}
}

// dumpCacheKey is "{provider}:{slug}". Provider names are fixed adapter ids.
func dumpCacheKey(provider, slug string) string {
	return provider + ":" + slug
}

// getOrLoadDump returns a cached board dump or loads and stores one.
// A nil receiver calls load(ctx) with no caching.
//
// Returned jobs and side are read-only (see dumpJob). The cache may share the
// same backing storage across callers; do not mutate. Callers that need to
// change fields (e.g. filling JD text the list endpoint omitted) must
// cloneDumpJobs first.
func (c *DumpCache) getOrLoadDump(
	ctx context.Context,
	provider, slug string,
	load func(context.Context) (jobs []dumpJob, side any, err error),
) (jobs []dumpJob, side any, err error) {
	if c == nil {
		return load(ctx)
	}
	key := dumpCacheKey(provider, slug)

	if e, ok := c.cache.GetIfPresent(key); ok && e != nil {
		return e.jobs, e.side, nil
	}

	jobs, side, err = load(ctx)
	if err != nil {
		return nil, nil, err
	}
	// Take ownership of the load result; load must not retain aliases.
	c.cache.Set(key, &dumpCacheEntry{jobs: jobs, side: side})
	return jobs, side, nil
}

// cloneDumpJobs returns a deep copy of jobs suitable for mutation by a
// caller that must not write through a dump()'s read-only return value.
func cloneDumpJobs(in []dumpJob) []dumpJob {
	out := slices.Clone(in)
	for i := range out {
		out[i].fields = cloneFields(out[i].fields)
	}
	return out
}

func cloneFields(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	out := maps.Clone(m)
	for k, v := range out {
		out[k] = slices.Clone(v)
	}
	return out
}
