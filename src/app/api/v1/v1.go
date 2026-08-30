package v1

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/z46-dev/sigil/service"
	"github.com/z46-dev/sigil/service/ipevaluation"
	"github.com/z46-dev/sigil/src/config"
	"github.com/z46-dev/sigil/src/db"
)

const (
	maximumSnapshotBytes int           = 64 * 1024
	maximumChallenges    int           = 10_000
	challengeLifetime    time.Duration = 2 * time.Minute
)

type (
	errorResponse struct {
		Error string `json:"error"`
	}

	challengeResponse struct {
		Challenge string    `json:"challenge"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	identifyRequest struct {
		Challenge string           `json:"challenge"`
		Snapshot  service.Snapshot `json:"snapshot"`
	}

	challengeRegistry struct {
		mutex      sync.Mutex
		challenges map[string]challengeRecord
	}

	challengeRecord struct {
		expiresAt time.Time
		binding   string
	}
)

var challenges = challengeRegistry{challenges: make(map[string]challengeRecord)}

func (registry *challengeRegistry) issue(now time.Time, binding string) (response challengeResponse, err error) {
	var random [24]byte

	if _, err = rand.Read(random[:]); err != nil {
		return
	}

	response = challengeResponse{
		Challenge: base64.RawURLEncoding.EncodeToString(random[:]),
		ExpiresAt: now.Add(challengeLifetime),
	}

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	for challenge, record := range registry.challenges {
		if !record.expiresAt.After(now) {
			delete(registry.challenges, challenge)
		}
	}

	if len(registry.challenges) >= maximumChallenges {
		err = fmt.Errorf("challenge capacity reached")
		return
	}

	registry.challenges[response.Challenge] = challengeRecord{expiresAt: response.ExpiresAt, binding: binding}
	return
}

func (registry *challengeRegistry) consume(challenge string, now time.Time, binding string) (valid bool) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	if record, exists := registry.challenges[challenge]; exists {
		delete(registry.challenges, challenge)
		valid = record.expiresAt.After(now) && hmac.Equal([]byte(record.binding), []byte(binding))
	}

	return
}

func networkPrefix(address string) (prefix []byte) {
	var ip net.IP = net.ParseIP(address)
	if ip == nil {
		return
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		prefix = ipv4.Mask(net.CIDRMask(24, 32))
	} else {
		prefix = ip.Mask(net.CIDRMask(56, 128))
	}

	return
}

func serverSignals(ctx fiber.Ctx) (signals service.ServerSignals) {
	signals.Protocol = ctx.Protocol()
	signals.TLS = strings.EqualFold(ctx.Scheme(), "https")
	if !config.Config.WebServer.ServerNetworkSignals {
		return
	}

	var prefix []byte = networkPrefix(ctx.IP())
	if len(prefix) == 0 {
		return
	}

	var mac hash.Hash = hmac.New(sha256.New, []byte(config.Config.WebServer.NetworkSignalKey))
	_, _ = mac.Write(prefix)
	signals.NetworkPrefixHash = hex.EncodeToString(mac.Sum(nil))

	var evaluation ipevaluation.Result
	if evaluation, _ = ipevaluation.Evaluate(ctx.IP()); evaluation.ASN != 0 || evaluation.CountryCode != "" ||
		evaluation.OpenProxy || evaluation.Tor || evaluation.Hosting || evaluation.Malicious {
		signals.IP = service.IPClassification{
			ASN:             evaluation.ASN,
			ASNOrganization: evaluation.ASNOrganization,
			CountryCode:     evaluation.CountryCode,
			City:            evaluation.City,
			OpenProxy:       evaluation.OpenProxy,
			Tor:             evaluation.Tor,
			Hosting:         evaluation.Hosting,
			Malicious:       evaluation.Malicious,
		}
	}
	return
}

func requestBinding(ctx fiber.Ctx) (binding string) {
	binding = ctx.Get(fiber.HeaderUserAgent)
	if signals := serverSignals(ctx); signals.NetworkPrefixHash != "" {
		binding += "\x00" + signals.NetworkPrefixHash
	}

	return
}

func sameOrigin(ctx fiber.Ctx) (allowed bool) {
	var (
		origin    *url.URL
		err       error
		originRaw string = ctx.Get(fiber.HeaderOrigin)
	)

	if originRaw == "" {
		return
	}

	if origin, err = url.Parse(originRaw); err == nil && (origin.Scheme == "http" || origin.Scheme == "https") {
		allowed = strings.EqualFold(origin.Host, ctx.Host())
	}

	return
}

func crossSite(ctx fiber.Ctx) (crossSiteRequest bool) {
	crossSiteRequest = strings.EqualFold(ctx.Get("Sec-Fetch-Site"), "cross-site")
	return
}

// Challenge issues a short-lived, single-use token for one identification attempt.
func Challenge(ctx fiber.Ctx) (err error) {
	var response challengeResponse

	if crossSite(ctx) {
		ctx.Status(fiber.StatusForbidden)
		err = ctx.JSON(errorResponse{Error: "cross-site challenge requests are not allowed"})
		return
	}

	if response, err = challenges.issue(time.Now().UTC(), requestBinding(ctx)); err != nil {
		ctx.Status(fiber.StatusInternalServerError)
		err = ctx.JSON(errorResponse{Error: "unable to issue challenge"})
		return
	}

	ctx.Set(fiber.HeaderCacheControl, "no-store")
	err = ctx.JSON(response)
	return
}

func decodeRequest(body []byte, target any) (err error) {
	var decoder *json.Decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	if err = decoder.Decode(target); err != nil {
		return
	}

	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("request must contain one JSON value")
		}

		return
	}

	err = nil
	return
}

// Identify validates, matches, and persists one browser observation.
func Identify(ctx fiber.Ctx) (err error) {
	var (
		request identifyRequest
		result  service.MatchResult
	)

	if !sameOrigin(ctx) {
		ctx.Status(fiber.StatusForbidden)
		err = ctx.JSON(errorResponse{Error: "identification requests must be same-origin"})
		return
	}

	if !ctx.IsJSON() {
		ctx.Status(fiber.StatusUnsupportedMediaType)
		err = ctx.JSON(errorResponse{Error: "content type must be application/json"})
		return
	}

	if len(ctx.Body()) == 0 || len(ctx.Body()) > maximumSnapshotBytes {
		ctx.Status(fiber.StatusRequestEntityTooLarge)
		err = ctx.JSON(errorResponse{Error: "snapshot body must be between 1 and 65536 bytes"})
		return
	}

	if err = decodeRequest(ctx.Body(), &request); err != nil {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: "invalid identification request"})
		return
	}

	if !challenges.consume(request.Challenge, time.Now().UTC(), requestBinding(ctx)) {
		ctx.Status(fiber.StatusUnauthorized)
		err = ctx.JSON(errorResponse{Error: "challenge is invalid, expired, or already used"})
		return
	}

	if request.Snapshot.Browser.UserAgent != ctx.Get(fiber.HeaderUserAgent) {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: "snapshot user agent does not match request"})
		return
	}

	if collectedAt := request.Snapshot.CollectedAt; collectedAt.Before(time.Now().UTC().Add(-5*time.Minute)) || collectedAt.After(time.Now().UTC().Add(time.Minute)) {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: "snapshot collection time is outside the accepted window"})
		return
	}

	if _, err = service.ValidateSnapshot(request.Snapshot); err != nil {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: err.Error()})
		return
	}

	request.Snapshot.Server = serverSignals(ctx)

	if result, err = db.MatchAndRecord(request.Snapshot); err != nil {
		ctx.Status(fiber.StatusInternalServerError)
		err = ctx.JSON(errorResponse{Error: "unable to match snapshot"})
		return
	}

	if result.Decision == "new" {
		ctx.Status(fiber.StatusCreated)
	} else {
		ctx.Status(fiber.StatusOK)
	}

	err = ctx.JSON(result)
	return
}

func Init(parent fiber.Router) {
	var v1Router = parent.Group("/v1")
	v1Router.Use(limiter.New(limiter.Config{
		Max:        30,
		Expiration: time.Minute,
	}))
	v1Router.Get("/challenge", Challenge)
	v1Router.Post("/identify", Identify)
}
