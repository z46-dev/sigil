package db

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/z46-dev/golog"
	"github.com/z46-dev/gosqlite"
	"github.com/z46-dev/sigil/service"
	"github.com/z46-dev/sigil/src/config"
)

var (
	driver       *gosqlite.Driver
	Fingerprints *gosqlite.RegisteredStruct[service.Fingerprint]
	Observations *gosqlite.RegisteredStruct[service.Observation]
	log          *golog.Logger
	matchMutex   sync.Mutex
)

func migrate[T any](table *gosqlite.RegisteredStruct[T], opts gosqlite.MigrationOptions) (report *gosqlite.MigrationReport, err error) {
	if report, err = table.Migrate(opts); err != nil {
		return
	}

	if len(report.AddedColumns) > 0 {
		log.Infof("Added columns to table %s: %v\n", table.Name, report.AddedColumns)
	}

	if len(report.DroppedColumns) > 0 {
		log.Infof("Dropped columns from table %s: %v\n", table.Name, report.DroppedColumns)
	}

	if len(report.ChangedColumns) > 0 {
		log.Infof("Changed columns in table %s: %v\n", table.Name, report.ChangedColumns)
	}

	if len(report.RenamedColumns) > 0 {
		log.Infof("Renamed columns in table %s: %v\n", table.Name, report.RenamedColumns)
	}

	if report.Rebuilt {
		log.Infof("Rebuilt table %s\n", table.Name)
	}

	log.Infof("Successfully migrated table %s\n", table.Name)
	return
}

func Init(l *golog.Logger) (err error) {
	log = l

	var migrationOptions = gosqlite.MigrationOptions{
		AllowDestructive: slices.ContainsFunc(os.Args, func(arg string) bool {
			return strings.Contains(arg, "--allow-destructive-migrations")
		}),
	}

	if driver, err = gosqlite.Begin(config.Config.Database.File); err != nil {
		err = fmt.Errorf("error open db: %w", err)
		return
	}

	if Fingerprints, err = gosqlite.Register(driver, service.Fingerprint{}); err != nil {
		err = fmt.Errorf("error registering fingerprints table: %w", err)
		return
	}

	if Observations, err = gosqlite.Register(driver, service.Observation{}); err != nil {
		err = fmt.Errorf("error registering observations table: %w", err)
		return
	}

	if _, err = migrate(Fingerprints, migrationOptions); err != nil {
		err = fmt.Errorf("error migrating fingerprints table: %w", err)
		return
	}

	if _, err = migrate(Observations, migrationOptions); err != nil {
		err = fmt.Errorf("error migrating observations table: %w", err)
	}

	return
}

func loadCandidates(mode service.Mode) (candidates []service.MatchCandidate, err error) {
	var observations []*service.Observation

	if observations, err = Observations.SelectAll(); err != nil {
		err = fmt.Errorf("list observations: %w", err)
		return
	}

	for _, observation := range observations {
		if observation.Mode != mode {
			continue
		}

		var signals service.BrowserSignals
		if err = json.Unmarshal([]byte(observation.SignalsJSON), &signals); err != nil {
			err = fmt.Errorf("decode observation %d: %w", observation.ID, err)
			return
		}

		candidates = append(candidates, service.MatchCandidate{
			VisitorID:  observation.VisitorID,
			SnapshotID: observation.SnapshotID,
			Signals:    signals,
		})
	}

	return
}

func updateVisitor(identifier, observedAt string) (err error) {
	var fingerprints []*service.Fingerprint

	if fingerprints, err = Fingerprints.SelectAll(); err != nil {
		return
	}

	for _, fingerprint := range fingerprints {
		if fingerprint.Identifier != identifier {
			continue
		}

		fingerprint.LastSeenAt = observedAt
		fingerprint.ObservationCount++
		err = Fingerprints.Update(fingerprint)
		return
	}

	err = fmt.Errorf("visitor %q was not found", identifier)
	return
}

// MatchAndRecord reconciles a validated snapshot and durably records non-ambiguous decisions.
func MatchAndRecord(snapshot service.Snapshot) (result service.MatchResult, err error) {
	matchMutex.Lock()
	defer matchMutex.Unlock()

	var (
		mode       service.Mode
		candidates []service.MatchCandidate
		encoded    []byte
		observedAt string = time.Now().UTC().Format(time.RFC3339Nano)
	)

	if mode, err = service.ValidateSnapshot(snapshot); err != nil {
		return
	}

	if candidates, err = loadCandidates(mode); err != nil {
		return
	}

	result = service.MatchSnapshot(snapshot, mode, candidates)
	if result.Decision == "ambiguous" {
		return
	}

	if result.Decision == "new" {
		var fingerprint service.Fingerprint

		if result.VisitorID, err = service.NewVisitorID(); err != nil {
			return
		}

		fingerprint = service.Fingerprint{
			Identifier:       result.VisitorID,
			Mode:             mode,
			CreatedAt:        observedAt,
			LastSeenAt:       observedAt,
			ObservationCount: 1,
		}

		if err = Fingerprints.Insert(&fingerprint); err != nil {
			err = fmt.Errorf("insert visitor: %w", err)
			return
		}
	} else if err = updateVisitor(result.VisitorID, observedAt); err != nil {
		return
	}

	if encoded, err = json.Marshal(snapshot.Browser); err != nil {
		return
	}

	if err = Observations.Insert(&service.Observation{
		VisitorID:   result.VisitorID,
		SnapshotID:  snapshot.SnapshotID,
		Mode:        mode,
		SignalsJSON: string(encoded),
		ObservedAt:  observedAt,
	}); err != nil {
		err = fmt.Errorf("insert observation: %w", err)
	}

	return
}
