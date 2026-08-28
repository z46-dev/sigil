package evaluation

import (
	"fmt"
	"time"

	"github.com/z46-dev/sigil/service"
)

type (
	// Observation is one labeled browser collection used by the replay evaluator.
	Observation struct {
		Label    string           `json:"label"`
		Browser  string           `json:"browser"`
		Scenario string           `json:"scenario"`
		Snapshot service.Snapshot `json:"snapshot"`
	}

	// Decision records the matcher output and whether it agreed with ground truth.
	Decision struct {
		Label       string              `json:"label"`
		Browser     string              `json:"browser"`
		Scenario    string              `json:"scenario"`
		ExpectedNew bool                `json:"expectedNew"`
		Correct     bool                `json:"correct"`
		Match       service.MatchResult `json:"match"`
	}

	// Metrics contains reproducible identification error counts and rates.
	Metrics struct {
		Observations      int     `json:"observations"`
		ReturningAttempts int     `json:"returningAttempts"`
		Correct           int     `json:"correct"`
		FalseMatches      int     `json:"falseMatches"`
		FalseNews         int     `json:"falseNews"`
		Ambiguous         int     `json:"ambiguous"`
		Accuracy          float64 `json:"accuracy"`
		FalseMatchRate    float64 `json:"falseMatchRate"`
		FalseNewRate      float64 `json:"falseNewRate"`
		AmbiguousRate     float64 `json:"ambiguousRate"`
	}

	// Report is the machine-readable output of one ordered longitudinal replay.
	Report struct {
		GeneratedAt time.Time  `json:"generatedAt"`
		Schema      int        `json:"schemaVersion"`
		Mode        string     `json:"mode"`
		Metrics     Metrics    `json:"metrics"`
		Decisions   []Decision `json:"decisions"`
		Limitations []string   `json:"limitations"`
	}
)

func ratio(numerator, denominator int) (value float64) {
	if denominator > 0 {
		value = float64(numerator) / float64(denominator)
	}

	return
}

// Evaluate replays labeled observations through the production matcher.
func Evaluate(mode service.Mode, observations []Observation) (report Report, err error) {
	var (
		candidates       []service.MatchCandidate
		visitorOwners    map[string]string = make(map[string]string)
		visitorsByLabel  map[string]string = make(map[string]string)
		nextVisitorIndex int
	)

	if mode != service.ModeDevice && mode != service.ModeBrowser && mode != service.ModeDeviceAndBrowser {
		err = fmt.Errorf("invalid evaluation mode %d", mode)
		return
	}

	report.GeneratedAt = time.Now().UTC()
	report.Schema = service.SchemaVersion
	report.Mode = mode.String()
	report.Limitations = []string{
		"Playwright browser engines are controlled builds, not every branded browser release.",
		"Emulated mobile contexts are synthetic negative controls, not separate physical hardware.",
		"CI runs measure repeatability on one runner and do not establish population accuracy.",
	}

	for _, observation := range observations {
		var (
			knownVisitor string
			known        bool
			decision     Decision
			assigned     string
		)

		knownVisitor, known = visitorsByLabel[observation.Label]
		decision = Decision{
			Label:       observation.Label,
			Browser:     observation.Browser,
			Scenario:    observation.Scenario,
			ExpectedNew: !known,
			Match:       service.MatchSnapshot(observation.Snapshot, mode, candidates),
		}

		report.Metrics.Observations++
		if known {
			report.Metrics.ReturningAttempts++
		}

		switch decision.Match.Decision {
		case "matched":
			assigned = decision.Match.VisitorID
			if visitorOwners[assigned] != observation.Label {
				report.Metrics.FalseMatches++
			} else {
				decision.Correct = true
			}
		case "ambiguous":
			report.Metrics.Ambiguous++
			if known {
				report.Metrics.FalseNews++
			}
		case "new":
			nextVisitorIndex++
			assigned = fmt.Sprintf("evaluation-%d", nextVisitorIndex)
			visitorOwners[assigned] = observation.Label
			if known {
				report.Metrics.FalseNews++
			} else {
				visitorsByLabel[observation.Label] = assigned
				decision.Correct = true
			}
		}

		if assigned != "" {
			candidates = append(candidates, service.MatchCandidate{
				VisitorID:  assigned,
				SnapshotID: observation.Snapshot.SnapshotID,
				Signals:    observation.Snapshot.Browser,
			})
		}

		if known && decision.Match.Decision == "matched" && assigned == knownVisitor {
			decision.Correct = true
		}

		if decision.Correct {
			report.Metrics.Correct++
		}

		report.Decisions = append(report.Decisions, decision)
	}

	report.Metrics.Accuracy = ratio(report.Metrics.Correct, report.Metrics.Observations)
	report.Metrics.FalseMatchRate = ratio(report.Metrics.FalseMatches, report.Metrics.Observations)
	report.Metrics.FalseNewRate = ratio(report.Metrics.FalseNews, report.Metrics.ReturningAttempts)
	report.Metrics.AmbiguousRate = ratio(report.Metrics.Ambiguous, report.Metrics.Observations)
	return
}
