package v1

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/z46-dev/sigil/service"
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
		userAgent string
	}
)

var challenges = challengeRegistry{challenges: make(map[string]challengeRecord)}

func (registry *challengeRegistry) issue(now time.Time, userAgent string) (response challengeResponse, err error) {
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

	registry.challenges[response.Challenge] = challengeRecord{expiresAt: response.ExpiresAt, userAgent: userAgent}
	return
}

func (registry *challengeRegistry) consume(challenge string, now time.Time, userAgent string) (valid bool) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	if record, exists := registry.challenges[challenge]; exists {
		delete(registry.challenges, challenge)
		valid = record.expiresAt.After(now) && record.userAgent == userAgent
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

	if response, err = challenges.issue(time.Now().UTC(), ctx.Get(fiber.HeaderUserAgent)); err != nil {
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

	if !challenges.consume(request.Challenge, time.Now().UTC(), ctx.Get(fiber.HeaderUserAgent)) {
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
