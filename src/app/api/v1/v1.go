package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/z46-dev/sigil/service"
	"github.com/z46-dev/sigil/src/db"
)

const maximumSnapshotBytes int = 64 * 1024

type errorResponse struct {
	Error string `json:"error"`
}

func decodeSnapshot(body []byte, snapshot *service.Snapshot) (err error) {
	var decoder *json.Decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	if err = decoder.Decode(snapshot); err != nil {
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
		snapshot service.Snapshot
		result   service.MatchResult
	)

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

	if err = decodeSnapshot(ctx.Body(), &snapshot); err != nil {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: "invalid snapshot JSON"})
		return
	}

	if _, err = service.ValidateSnapshot(snapshot); err != nil {
		ctx.Status(fiber.StatusBadRequest)
		err = ctx.JSON(errorResponse{Error: err.Error()})
		return
	}

	if result, err = db.MatchAndRecord(snapshot); err != nil {
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
	v1Router.Post("/identify", Identify)
}
