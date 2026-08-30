package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"
)

type (
	// MatchCandidate is one historical observation eligible for matching.
	MatchCandidate struct {
		VisitorID  string
		SnapshotID string
		Signals    BrowserSignals
		Server     ServerSignals
	}

	// SignalEvidence explains one contribution to a candidate score.
	SignalEvidence struct {
		Name       string  `json:"name"`
		Weight     float64 `json:"weight"`
		Similarity float64 `json:"similarity"`
	}

	// CandidateScore summarizes the best historical observation for one visitor.
	CandidateScore struct {
		VisitorID     string           `json:"visitorId"`
		Score         float64          `json:"score"`
		Coverage      float64          `json:"coverage"`
		Evidence      []SignalEvidence `json:"evidence,omitempty"`
		SnapshotMatch bool             `json:"snapshotMatch"`
	}

	// MatchResult reports a conservative identity decision and collision measurements.
	MatchResult struct {
		Decision         string            `json:"decision"`
		VisitorID        string            `json:"visitorId,omitempty"`
		Confidence       float64           `json:"confidence"`
		BestScore        float64           `json:"bestScore"`
		RunnerUpScore    float64           `json:"runnerUpScore"`
		Margin           float64           `json:"margin"`
		EvidenceCoverage float64           `json:"evidenceCoverage"`
		Collision        bool              `json:"collision"`
		CandidateCount   int               `json:"candidateCount"`
		Evidence         []SignalEvidence  `json:"evidence,omitempty"`
		Aggressiveness   Aggressiveness    `json:"aggressiveness"`
		IP               *IPClassification `json:"ip,omitempty"`
	}

	matcherScore struct {
		matchedWeight float64
		presentWeight float64
		evidence      []SignalEvidence
	}
)

const (
	matchThreshold      float64 = 0.80
	confidenceThreshold float64 = 0.70
	coverageThreshold   float64 = 0.45
	collisionMargin     float64 = 0.08
)

func clamp(value float64) (result float64) {
	result = math.Max(0, math.Min(1, value))
	return
}

func numericSimilarity(first, second, tolerance float64) (similarity float64) {
	if first == second {
		similarity = 1
		return
	}

	if tolerance > 0 {
		similarity = clamp(1 - math.Abs(first-second)/tolerance)
	}

	return
}

func (score *matcherScore) add(name string, weight float64, available bool, similarity float64) {
	if !available {
		return
	}

	similarity = clamp(similarity)
	score.presentWeight += weight
	score.matchedWeight += weight * similarity
	score.evidence = append(score.evidence, SignalEvidence{Name: name, Weight: weight, Similarity: similarity})
}

func sameString(first, second string) (similarity float64) {
	if first == second {
		similarity = 1
	}

	return
}

func sameStrings(first, second []string) (similarity float64) {
	if strings.Join(first, "\x00") == strings.Join(second, "\x00") {
		similarity = 1
	}

	return
}

func addDeviceEvidence(score *matcherScore, current, previous BrowserSignals) {
	var (
		currentShort, currentLong   int
		previousShort, previousLong int
	)

	currentShort, currentLong = orderedDimensions(current.ScreenWidth, current.ScreenHeight)
	previousShort, previousLong = orderedDimensions(previous.ScreenWidth, previous.ScreenHeight)
	score.add("platform", 14, current.Platform != "" && previous.Platform != "", sameString(current.Platform, previous.Platform))
	score.add("screen-short-side", 5, currentShort > 0 && previousShort > 0, numericSimilarity(float64(currentShort), float64(previousShort), 160))
	score.add("screen-long-side", 5, currentLong > 0 && previousLong > 0, numericSimilarity(float64(currentLong), float64(previousLong), 240))
	score.add("color-depth", 2, current.ColorDepth > 0 && previous.ColorDepth > 0, sameString(fmt.Sprint(current.ColorDepth), fmt.Sprint(previous.ColorDepth)))
	score.add("pixel-ratio", 3, current.PixelRatio > 0 && previous.PixelRatio > 0, numericSimilarity(current.PixelRatio, previous.PixelRatio, 0.5))
	score.add("cpu-count", 7, current.HardwareConcurrency > 0 && previous.HardwareConcurrency > 0, numericSimilarity(float64(current.HardwareConcurrency), float64(previous.HardwareConcurrency), 4))
	score.add("device-memory", 6, current.DeviceMemory > 0 && previous.DeviceMemory > 0, numericSimilarity(current.DeviceMemory, previous.DeviceMemory, 4))
	score.add("touch-points", 5, current.MaxTouchPoints >= 0 && previous.MaxTouchPoints >= 0, numericSimilarity(float64(current.MaxTouchPoints), float64(previous.MaxTouchPoints), 2))
	score.add("webgl-vendor", 7, current.Rendering.WebGLVendor != "" && previous.Rendering.WebGLVendor != "", sameString(current.Rendering.WebGLVendor, previous.Rendering.WebGLVendor))
	score.add("webgl-renderer", 14, current.Rendering.WebGLRenderer != "" && previous.Rendering.WebGLRenderer != "", sameString(current.Rendering.WebGLRenderer, previous.Rendering.WebGLRenderer))
	score.add("font-set", 10, current.Fonts.DetectedHash != "" && previous.Fonts.DetectedHash != "", sameString(current.Fonts.DetectedHash, previous.Fonts.DetectedHash))
}

func addBrowserEvidence(score *matcherScore, current, previous BrowserSignals) {
	score.add("user-agent", 10, current.UserAgent != "" && previous.UserAgent != "", sameString(current.UserAgent, previous.UserAgent))
	score.add("vendor", 4, current.Vendor != "" && previous.Vendor != "", sameString(current.Vendor, previous.Vendor))
	score.add("languages", 4, len(current.Languages) > 0 && len(previous.Languages) > 0, sameStrings(current.Languages, previous.Languages))
	score.add("timezone", 3, current.Timezone != "" && previous.Timezone != "", sameString(current.Timezone, previous.Timezone))
	score.add("canvas", 12, current.Rendering.CanvasHash != "" && previous.Rendering.CanvasHash != "", sameString(current.Rendering.CanvasHash, previous.Rendering.CanvasHash))
	score.add("webgl-extensions", 6, current.Rendering.WebGLExtensionsHash != "" && previous.Rendering.WebGLExtensionsHash != "", sameString(current.Rendering.WebGLExtensionsHash, previous.Rendering.WebGLExtensionsHash))
	score.add("audio", 9, current.Audio.Hash != "" && previous.Audio.Hash != "", sameString(current.Audio.Hash, previous.Audio.Hash))
	score.add("font-metrics", 7, current.Fonts.MetricsHash != "" && previous.Fonts.MetricsHash != "", sameString(current.Fonts.MetricsHash, previous.Fonts.MetricsHash))
}

func addServerEvidence(score *matcherScore, current, previous ServerSignals) {
	score.add("network-prefix", 8, current.NetworkPrefixHash != "" && previous.NetworkPrefixHash != "", sameString(current.NetworkPrefixHash, previous.NetworkPrefixHash))
	score.add("asn", 3, current.IP.ASN != 0 && previous.IP.ASN != 0, sameString(fmt.Sprint(current.IP.ASN), fmt.Sprint(previous.IP.ASN)))
}

func maximumWeight(mode Mode) (weight float64) {
	if mode&ModeDevice != 0 {
		weight += 78
	}

	if mode&ModeBrowser != 0 {
		weight += 55
	}

	return
}

// CalculateAggressiveness estimates privacy impact from the available fingerprint inputs.
func CalculateAggressiveness(snapshot Snapshot, mode Mode) (result Aggressiveness) {
	var score matcherScore

	if mode&ModeDevice != 0 {
		addDeviceEvidence(&score, snapshot.Browser, snapshot.Browser)
	}

	if mode&ModeBrowser != 0 {
		addBrowserEvidence(&score, snapshot.Browser, snapshot.Browser)
	}

	addServerEvidence(&score, snapshot.Server, snapshot.Server)
	score.add("ip-country", 2, snapshot.Server.IP.CountryCode != "", 1)
	score.add("ip-city", 4, snapshot.Server.IP.City != "", 1)
	result.SignalCount = len(score.evidence)
	result.Score = int(math.Round(clamp(score.presentWeight/(maximumWeight(ModeDeviceAndBrowser)+17)) * 100))
	result.Level = "low"
	if result.Score >= 70 {
		result.Level = "high"
	} else if result.Score >= 35 {
		result.Level = "moderate"
	}

	return
}

func scoreCandidate(snapshot Snapshot, candidate MatchCandidate, mode Mode) (result CandidateScore) {
	var (
		score   matcherScore
		maximum float64 = maximumWeight(mode)
	)

	if mode&ModeDevice != 0 {
		addDeviceEvidence(&score, snapshot.Browser, candidate.Signals)
	}

	if mode&ModeBrowser != 0 {
		addBrowserEvidence(&score, snapshot.Browser, candidate.Signals)
	}

	addServerEvidence(&score, snapshot.Server, candidate.Server)
	if snapshot.Server.NetworkPrefixHash != "" {
		maximum += 8
	}
	if snapshot.Server.IP.ASN != 0 {
		maximum += 3
	}

	result = CandidateScore{
		VisitorID:     candidate.VisitorID,
		Coverage:      clamp(score.presentWeight / maximum),
		Evidence:      score.evidence,
		SnapshotMatch: snapshot.SnapshotID == candidate.SnapshotID,
	}

	if score.presentWeight > 0 {
		result.Score = clamp(score.matchedWeight / score.presentWeight)
	}

	if result.SnapshotMatch {
		result.Score = 1
	}

	return
}

// MatchSnapshot compares an observation with visitor histories and refuses ambiguous merges.
func MatchSnapshot(snapshot Snapshot, mode Mode, candidates []MatchCandidate) (result MatchResult) {
	var bestByVisitor map[string]CandidateScore = make(map[string]CandidateScore)

	for _, candidate := range candidates {
		var score CandidateScore = scoreCandidate(snapshot, candidate, mode)

		if previous, exists := bestByVisitor[candidate.VisitorID]; !exists || score.Score > previous.Score {
			bestByVisitor[candidate.VisitorID] = score
		}
	}

	var ranked []CandidateScore = make([]CandidateScore, 0, len(bestByVisitor))
	for _, score := range bestByVisitor {
		ranked = append(ranked, score)
	}

	sort.Slice(ranked, func(first, second int) bool { return ranked[first].Score > ranked[second].Score })

	result.Decision = "new"
	result.Aggressiveness = CalculateAggressiveness(snapshot, mode)
	if snapshot.Server.IP.ASN != 0 || snapshot.Server.IP.CountryCode != "" {
		result.IP = &snapshot.Server.IP
	}
	result.CandidateCount = len(ranked)
	result.Margin = 1
	if len(ranked) == 0 {
		return
	}

	result.BestScore = ranked[0].Score
	result.EvidenceCoverage = ranked[0].Coverage
	result.Evidence = ranked[0].Evidence
	if len(ranked) > 1 {
		result.RunnerUpScore = ranked[1].Score
		result.Margin = clamp(ranked[0].Score - ranked[1].Score)
	}

	result.Confidence = clamp(result.BestScore * (0.6 + 0.4*result.EvidenceCoverage))
	if len(ranked) > 1 && result.Margin < collisionMargin {
		result.Confidence *= result.Margin / collisionMargin
		result.Collision = ranked[0].Score >= matchThreshold && ranked[1].Score >= matchThreshold-collisionMargin
	}

	if result.Collision {
		result.Decision = "ambiguous"
	} else if result.BestScore >= matchThreshold && result.EvidenceCoverage >= coverageThreshold && result.Confidence >= confidenceThreshold {
		result.Decision = "matched"
		result.VisitorID = ranked[0].VisitorID
	}

	return
}

// NewVisitorID creates an opaque random server-side identifier.
func NewVisitorID() (identifier string, err error) {
	var random [18]byte

	if _, err = rand.Read(random[:]); err != nil {
		err = fmt.Errorf("generate visitor identifier: %w", err)
		return
	}

	identifier = "sv1_" + base64.RawURLEncoding.EncodeToString(random[:])
	return
}
