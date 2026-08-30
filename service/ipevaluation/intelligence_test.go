//go:build !js

package ipevaluation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTextFeedNormalizes verifies proxy endpoints and CIDRs become unique public prefixes.
func TestTextFeedNormalizes(t *testing.T) {
	var server *httptest.Server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte("http://8.8.8.8:8080\nsocks5://8.8.8.8:3128\n1.1.1.0/24\n10.0.0.1:80\n# comment\n"))
	}))
	defer server.Close()

	var (
		fetch      feedFetcher = textFeed(server.Client(), server.URL, CategoryOpenProxy, 0.65)
		indicators []Indicator
		err        error
	)
	if indicators, err = fetch(context.Background()); err != nil {
		t.Fatalf("fetch text feed: %v", err)
	}

	if len(indicators) != 2 {
		t.Fatalf("normalized indicators = %+v", indicators)
	}
}

// TestLivePublicFeeds validates configured public source formats when explicitly requested.
func TestLivePublicFeeds(t *testing.T) {
	if os.Getenv("SIGIL_LIVE_FEED_TESTS") == "" {
		t.Skip("live feed tests are disabled")
	}

	for _, source := range defaultSources("") {
		t.Run(source.name, func(t *testing.T) {
			var (
				indicators []Indicator
				err        error
			)
			if indicators, err = source.fetch(context.Background()); err != nil {
				t.Fatalf("fetch source: %v", err)
			}
			if len(indicators) == 0 {
				t.Fatal("source returned no indicators")
			}
		})
	}
}

// TestFeedStoreBuildsOverlappingIndexes verifies the separate cache preserves independent classifications.
func TestFeedStoreBuildsOverlappingIndexes(t *testing.T) {
	var (
		store      *feedStore
		index      *indicatorIndex
		err        error
		now        time.Time  = time.Now().UTC()
		proxyFeed  feedSource = feedSource{name: "proxy", interval: time.Hour, ttl: time.Hour}
		threatFeed feedSource = feedSource{name: "threat", interval: time.Hour, ttl: time.Hour}
	)

	if store, err = openFeedStore(filepath.Join(t.TempDir(), "intelligence.db")); err != nil {
		t.Fatalf("open feed store: %v", err)
	}
	defer store.database.Close()

	if err = store.replace(proxyFeed, []Indicator{{Prefix: netip.MustParsePrefix("8.8.8.0/24"), Category: CategoryOpenProxy, Confidence: 0.6}}, now); err != nil {
		t.Fatalf("store proxy feed: %v", err)
	}
	if err = store.replace(threatFeed, []Indicator{{Prefix: netip.MustParsePrefix("8.8.8.8/32"), Category: CategoryMalicious, Confidence: 0.9}}, now); err != nil {
		t.Fatalf("store threat feed: %v", err)
	}

	if index, err = store.load(now); err != nil {
		t.Fatalf("load indicator index: %v", err)
	}

	var result Intelligence = index.evaluate(netip.MustParseAddr("8.8.8.8"))
	if !result.OpenProxy || !result.Malicious || result.Tor || result.Hosting {
		t.Fatalf("overlapping classifications were lost: %+v", result)
	}
}

// TestEmptyUpdateRetainsLastGoodSnapshot verifies a broken feed cannot erase cached indicators.
func TestEmptyUpdateRetainsLastGoodSnapshot(t *testing.T) {
	var (
		store   *feedStore
		service feedService
		err     error
		now     time.Time  = time.Now().UTC()
		source  feedSource = feedSource{
			name:     "proxy",
			interval: time.Hour,
			ttl:      time.Hour,
			fetch:    func(context.Context) ([]Indicator, error) { return nil, nil },
		}
	)

	if store, err = openFeedStore(filepath.Join(t.TempDir(), "intelligence.db")); err != nil {
		t.Fatalf("open feed store: %v", err)
	}
	defer store.database.Close()
	service.store = store
	service.index.Store(newIndicatorIndex())

	if err = store.replace(source, []Indicator{{Prefix: netip.MustParsePrefix("8.8.8.8/32"), Category: CategoryOpenProxy, Confidence: 0.6}}, now); err != nil {
		t.Fatalf("store initial feed: %v", err)
	}
	service.update(context.Background(), source)

	var count int
	if err = store.database.QueryRow(`SELECT COUNT(*) FROM indicators WHERE source = ?`, source.name).Scan(&count); err != nil {
		t.Fatalf("count retained indicators: %v", err)
	}
	if count != 1 {
		t.Fatalf("empty update replaced the last good snapshot: count=%d", count)
	}
}
