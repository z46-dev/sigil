//go:build !js

package ipevaluation

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gaissmai/bart"
	_ "modernc.org/sqlite"
)

type (
	Category string

	Indicator struct {
		Prefix     netip.Prefix
		Category   Category
		Confidence float64
	}

	Intelligence struct {
		OpenProxy bool
		Tor       bool
		Hosting   bool
		Malicious bool
	}

	feedSource struct {
		name     string
		interval time.Duration
		ttl      time.Duration
		fetch    feedFetcher
	}

	feedFetcher func(context.Context) ([]Indicator, error)

	indicatorIndex struct {
		categories map[Category]*bart.Lite
	}

	feedStore struct {
		database *sql.DB
	}

	feedService struct {
		store   *feedStore
		sources []feedSource
		index   atomic.Pointer[indicatorIndex]
		cancel  context.CancelFunc
		wait    sync.WaitGroup
	}
)

const (
	CategoryOpenProxy Category = "open-proxy"
	CategoryTor       Category = "tor"
	CategoryHosting   Category = "hosting"
	CategoryMalicious Category = "malicious"
)

func (index *indicatorIndex) evaluate(address netip.Addr) (result Intelligence) {
	address = address.Unmap()
	result.OpenProxy = index.categories[CategoryOpenProxy].Contains(address)
	result.Tor = index.categories[CategoryTor].Contains(address)
	result.Hosting = index.categories[CategoryHosting].Contains(address)
	result.Malicious = index.categories[CategoryMalicious].Contains(address)
	return
}

func newIndicatorIndex() (index *indicatorIndex) {
	index = &indicatorIndex{categories: map[Category]*bart.Lite{
		CategoryOpenProxy: {},
		CategoryTor:       {},
		CategoryHosting:   {},
		CategoryMalicious: {},
	}}
	return
}

func openFeedStore(path string) (store *feedStore, err error) {
	store = &feedStore{}
	if store.database, err = sql.Open("sqlite", path); err != nil {
		return
	}

	store.database.SetMaxOpenConns(1)
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`CREATE TABLE IF NOT EXISTS indicators (
            source TEXT NOT NULL,
            prefix TEXT NOT NULL,
            category TEXT NOT NULL,
            confidence REAL NOT NULL,
			observed_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
            PRIMARY KEY (source, prefix, category)
        )`,
		`CREATE INDEX IF NOT EXISTS indicators_expiry ON indicators (expires_at)`,
		`CREATE TABLE IF NOT EXISTS feed_status (
            source TEXT PRIMARY KEY,
			updated_at INTEGER NOT NULL,
			next_update_at INTEGER NOT NULL,
            last_error TEXT NOT NULL,
            indicator_count INTEGER NOT NULL
        )`,
	} {
		if _, err = store.database.Exec(statement); err != nil {
			_ = store.database.Close()
			store = nil
			return
		}
	}

	return
}

func (store *feedStore) replace(source feedSource, indicators []Indicator, now time.Time) (err error) {
	var transaction *sql.Tx
	if transaction, err = store.database.Begin(); err != nil {
		return
	}
	defer transaction.Rollback()

	if _, err = transaction.Exec(`DELETE FROM indicators WHERE source = ?`, source.name); err != nil {
		return
	}

	var statement *sql.Stmt
	if statement, err = transaction.Prepare(`INSERT INTO indicators
        (source, prefix, category, confidence, observed_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?)`); err != nil {
		return
	}
	defer statement.Close()

	for _, indicator := range indicators {
		if _, err = statement.Exec(source.name, indicator.Prefix.String(), indicator.Category, indicator.Confidence,
			now.Unix(), now.Add(source.ttl).Unix()); err != nil {
			return
		}
	}

	if _, err = transaction.Exec(`INSERT INTO feed_status
        (source, updated_at, next_update_at, last_error, indicator_count)
        VALUES (?, ?, ?, '', ?)
        ON CONFLICT(source) DO UPDATE SET
            updated_at = excluded.updated_at,
            next_update_at = excluded.next_update_at,
            last_error = '',
            indicator_count = excluded.indicator_count`,
		source.name, now.Unix(), now.Add(source.interval).Unix(), len(indicators)); err != nil {
		return
	}

	err = transaction.Commit()
	return
}

func (store *feedStore) recordFailure(source feedSource, now time.Time, updateErr error) {
	_, _ = store.database.Exec(`INSERT INTO feed_status
        (source, updated_at, next_update_at, last_error, indicator_count)
        VALUES (?, '', ?, ?, 0)
        ON CONFLICT(source) DO UPDATE SET
            next_update_at = excluded.next_update_at,
            last_error = excluded.last_error`,
		source.name, now.Add(min(source.interval/4, 15*time.Minute)).Unix(), updateErr.Error())
}

func (store *feedStore) due(source feedSource, now time.Time) (due bool) {
	var (
		nextUnix int64
		err      error
	)
	if err = store.database.QueryRow(`SELECT next_update_at FROM feed_status WHERE source = ?`, source.name).Scan(&nextUnix); err != nil {
		due = err == sql.ErrNoRows
		return
	}

	if nextUnix <= now.Unix() {
		due = true
	}
	return
}

func (store *feedStore) load(now time.Time) (index *indicatorIndex, err error) {
	var rows *sql.Rows
	if rows, err = store.database.Query(`SELECT prefix, category FROM indicators WHERE expires_at > ?`, now.Unix()); err != nil {
		return
	}
	defer rows.Close()

	index = newIndicatorIndex()
	for rows.Next() {
		var (
			prefixRaw string
			category  Category
			prefix    netip.Prefix
		)

		if err = rows.Scan(&prefixRaw, &category); err != nil {
			return
		}

		if prefix, err = netip.ParsePrefix(prefixRaw); err != nil {
			err = fmt.Errorf("decode stored prefix %q: %w", prefixRaw, err)
			return
		}

		if table, exists := index.categories[category]; exists {
			table.Insert(prefix)
		}
	}

	err = rows.Err()
	return
}

func (service *feedService) rebuild() (err error) {
	var index *indicatorIndex
	if index, err = service.store.load(time.Now().UTC()); err == nil {
		service.index.Store(index)
	}
	return
}

func (service *feedService) update(ctx context.Context, source feedSource) (rebuild bool) {
	var (
		indicators []Indicator
		err        error
		now        time.Time = time.Now().UTC()
	)

	if indicators, err = source.fetch(ctx); err != nil {
		service.store.recordFailure(source, now, err)
		log.Printf("IP intelligence source %s failed: %v", source.name, err)
		rebuild = true
		return
	}

	if len(indicators) == 0 {
		service.store.recordFailure(source, now, fmt.Errorf("source returned no indicators"))
		log.Printf("IP intelligence source %s returned no indicators", source.name)
		rebuild = true
		return
	}

	if err = service.store.replace(source, indicators, now); err != nil {
		service.store.recordFailure(source, now, err)
		log.Printf("IP intelligence source %s could not be stored: %v", source.name, err)
		return
	}

	log.Printf("IP intelligence source %s updated with %d indicators", source.name, len(indicators))
	rebuild = true
	return
}

func (service *feedService) run(ctx context.Context) {
	defer service.wait.Done()
	var timer *time.Timer = time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			var (
				rebuild    bool
				rebuildErr error
			)
			for _, source := range service.sources {
				if service.store.due(source, time.Now().UTC()) {
					rebuild = service.update(ctx, source) || rebuild
				}
			}
			if rebuild {
				if rebuildErr = service.rebuild(); rebuildErr != nil {
					log.Printf("IP intelligence index rebuild failed: %v", rebuildErr)
				}
			}
			timer.Reset(time.Minute)
		}
	}
}

func (service *feedService) close() {
	if service == nil {
		return
	}

	if service.cancel != nil {
		service.cancel()
	}
	service.wait.Wait()
	if service.store != nil {
		_ = service.store.database.Close()
	}
}

func startFeedService(databasePath, threatFoxAuthKey string) (service *feedService, err error) {
	service = &feedService{sources: defaultSources(threatFoxAuthKey)}
	if service.store, err = openFeedStore(databasePath); err != nil {
		service = nil
		return
	}

	if err = service.rebuild(); err != nil {
		_ = service.store.database.Close()
		service = nil
		return
	}

	var ctx context.Context
	ctx, service.cancel = context.WithCancel(context.Background())
	service.wait.Add(1)
	go service.run(ctx)
	return
}
