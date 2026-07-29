package ats

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleJobs(n int, desc string) []dumpJob {
	out := make([]dumpJob, n)
	for i := range out {
		out[i] = dumpJob{
			summary: JobSummary{
				JobID: fmt.Sprintf("id-%d", i),
				Title: fmt.Sprintf("title-%d", i),
			},
			description: desc,
			fields: map[string][]string{
				"team": {fmt.Sprintf("t-%d", i)},
			},
		}
	}
	return out
}

func TestDumpCacheHitMiss(t *testing.T) {
	var loads atomic.Int32
	c := NewDumpCache(DumpCacheConfig{TTL: time.Minute})
	load := func(context.Context) ([]dumpJob, any, error) {
		loads.Add(1)
		return sampleJobs(2, "jd"), "side", nil
	}
	ctx := context.Background()
	j1, s1, err := c.getOrLoadDump(ctx, "greenhouse", "acme", load)
	require.NoError(t, err)
	assert.Equal(t, int32(1), loads.Load())
	assert.Equal(t, "side", s1)
	assert.Len(t, j1, 2)

	_, _, err = c.getOrLoadDump(ctx, "greenhouse", "acme", load)
	require.NoError(t, err)
	assert.Equal(t, int32(1), loads.Load(), "second call should hit cache")
}

func TestDumpCacheNilSkipsCaching(t *testing.T) {
	var loads atomic.Int32
	load := func(context.Context) ([]dumpJob, any, error) {
		loads.Add(1)
		return sampleJobs(1, "jd"), nil, nil
	}
	ctx := context.Background()
	var nilCache *DumpCache
	_, _, err := nilCache.getOrLoadDump(ctx, "p", "x", load)
	require.NoError(t, err)
	_, _, err = nilCache.getOrLoadDump(ctx, "p", "x", load)
	require.NoError(t, err)
	assert.Equal(t, int32(2), loads.Load())
}

func TestDumpCacheEmptyAndErrors(t *testing.T) {
	c := NewDumpCache(DumpCacheConfig{TTL: time.Hour})
	ctx := context.Background()
	var loads atomic.Int32
	_, _, err := c.getOrLoadDump(ctx, "p", "empty", func(context.Context) ([]dumpJob, any, error) {
		loads.Add(1)
		return []dumpJob{}, nil, nil
	})
	require.NoError(t, err)
	_, _, err = c.getOrLoadDump(ctx, "p", "empty", func(context.Context) ([]dumpJob, any, error) {
		loads.Add(1)
		return []dumpJob{}, nil, nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), loads.Load(), "empty board should be cached")

	boom := errors.New("upstream")
	loads.Store(0)
	for range 2 {
		_, _, err := c.getOrLoadDump(ctx, "p", "err", func(context.Context) ([]dumpJob, any, error) {
			loads.Add(1)
			return nil, nil, boom
		})
		assert.ErrorIs(t, err, boom)
	}
	assert.Equal(t, int32(2), loads.Load(), "errors must not be cached")
}

func TestCloneDumpJobsIsolation(t *testing.T) {
	// Callers that must mutate (e.g. BambooHR enrich) clone first; the
	// original dump / cache entry stays unchanged.
	orig := sampleJobs(1, "orig")
	cp := cloneDumpJobs(orig)
	cp[0].description = "mutated"
	cp[0].fields["team"][0] = "mutated-team"
	assert.Equal(t, "orig", orig[0].description)
	assert.Equal(t, "t-0", orig[0].fields["team"][0])
}

func TestDumpCacheKeyUsesColon(t *testing.T) {
	assert.Equal(t, "greenhouse:acme", dumpCacheKey("greenhouse", "acme"))
}

func TestDumpCacheDefaults(t *testing.T) {
	c := NewDumpCache(DumpCacheConfig{})
	assert.Equal(t, DefaultDumpCacheTTL, c.cfg.TTL)
	assert.Equal(t, DefaultDumpCacheMaxEntries, c.cfg.MaxEntries)
	assert.Equal(t, DefaultDumpCacheMaxEntries, 100)
}
