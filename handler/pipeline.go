package handler

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/usecase"
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
		slog.Error("handler: pipeline failed", "name", name, "err", err, "latency", time.Since(start))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "unknown output type"})
	}
}
