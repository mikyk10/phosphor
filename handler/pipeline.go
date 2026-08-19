package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/usecase"
)

type PipelineHandler struct {
	uc usecase.PipelineUsecase
}

func NewPipelineHandler(uc usecase.PipelineUsecase) *PipelineHandler {
	return &PipelineHandler{uc: uc}
}

// Run handles GET|POST /pipeline/:name
func (h *PipelineHandler) Run(c *echo.Context) error {
	name := c.Param("name")

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
	}

	input := usecase.PipelineInput{
		PipelineName: name,
		Size:         c.QueryParam("size"),
		Quality:      c.QueryParam("quality"),
	}
	if len(body) > 0 {
		input.SourceImage = body
	}
	if mt := c.QueryParam("max_tags"); mt != "" {
		v, err := strconv.Atoi(mt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid max_tags: " + mt})
		}
		input.MaxTags = v
	}

	slog.Info("handler: pipeline", "method", c.Request().Method, "name", name, "size", input.Size, "quality", input.Quality, "body_bytes", len(body))
	start := time.Now()

	result, err := h.uc.Run(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrPipelineNotFound) {
			slog.Warn("handler: pipeline not found", "name", name, "latency", time.Since(start))
			return c.JSON(http.StatusNotFound, map[string]string{"error": "pipeline not found"})
		}
		// The underlying error can carry provider response bodies and local
		// filesystem paths, so only a correlation ID goes to the client.
		errorID := newErrorID()
		slog.Error("handler: pipeline failed", "name", name, "error_id", errorID, "err", err, "latency", time.Since(start))
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":    "pipeline execution failed",
			"error_id": errorID,
		})
	}

	slog.Info("handler: pipeline completed", "name", name, "output_type", result.OutputType, "latency", time.Since(start))

	switch result.OutputType {
	case "image":
		c.Response().Header().Set("Content-Type", result.ContentType)
		c.Response().WriteHeader(http.StatusOK)
		n, writeErr := c.Response().Write(result.ImageData)
		if writeErr != nil {
			slog.Error("handler: failed to write image response", "bytes_written", n, "err", writeErr)
			return writeErr
		}
		return nil
	case "text":
		if result.Tags != nil {
			return c.JSON(http.StatusOK, map[string]any{"tags": result.Tags})
		}
		return c.JSON(http.StatusOK, map[string]any{"text": result.Text})
	default:
		errorID := newErrorID()
		slog.Error("handler: unknown output type", "name", name, "error_id", errorID, "output_type", result.OutputType)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":    "pipeline execution failed",
			"error_id": errorID,
		})
	}
}

// newErrorID returns a short correlation ID that ties the client's 500 to the
// full error in the server log.
func newErrorID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on supported platforms; fall back to a
		// timestamp rather than dropping correlation entirely.
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b[:])
}
