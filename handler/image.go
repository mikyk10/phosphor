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

type ImageHandler struct {
	generate usecase.GenerateUsecase
	remix    usecase.RemixUsecase
}

func NewImageHandler(generate usecase.GenerateUsecase, remix usecase.RemixUsecase) *ImageHandler {
	return &ImageHandler{generate: generate, remix: remix}
}

// Generate handles GET /image — text-to-image generation.
func (h *ImageHandler) Generate(c *echo.Context) error {
	input := usecase.GenerateInput{
		PipelineName: c.QueryParam("pipeline"),
		Orientation:  c.QueryParam("orientation"),
		Quality:      c.QueryParam("quality"),
	}
	if w := c.QueryParam("width"); w != "" {
		v, err := strconv.Atoi(w)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid width: " + w})
		}
		input.Width = v
	}
	if ht := c.QueryParam("height"); ht != "" {
		v, err := strconv.Atoi(ht)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid height: " + ht})
		}
		input.Height = v
	}

	slog.Info("handler: GET /image", "pipeline", input.PipelineName, "width", input.Width, "height", input.Height, "orientation", input.Orientation, "quality", input.Quality)
	start := time.Now()

	result, err := h.generate.Run(c.Request().Context(), input)
	if err != nil {
		slog.Error("handler: GET /image failed", "err", err, "latency", time.Since(start))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	slog.Info("handler: GET /image completed", "content_type", result.ContentType, "bytes", len(result.ImageData), "latency", time.Since(start))

	c.Response().Header().Set("Content-Type", result.ContentType)
	c.Response().WriteHeader(http.StatusOK)
	n, writeErr := c.Response().Write(result.ImageData)
	if writeErr != nil {
		slog.Error("handler: failed to write response", "bytes_written", n, "err", writeErr)
		return writeErr
	}
	return nil
}

// Remix handles POST /image — image-to-image processing.
func (h *ImageHandler) Remix(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
	}
	if len(body) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "request body is empty"})
	}

	input := usecase.RemixInput{
		PipelineName: c.QueryParam("pipeline"),
		SourceImage:  body,
		Quality:      c.QueryParam("quality"),
	}
	if w := c.QueryParam("width"); w != "" {
		v, err := strconv.Atoi(w)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid width: " + w})
		}
		input.Width = v
	}
	if ht := c.QueryParam("height"); ht != "" {
		v, err := strconv.Atoi(ht)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid height: " + ht})
		}
		input.Height = v
	}

	slog.Info("handler: POST /image", "pipeline", input.PipelineName, "body_bytes", len(body), "quality", input.Quality)
	start := time.Now()

	result, err := h.remix.Run(c.Request().Context(), input)
	if err != nil {
		slog.Error("handler: POST /image failed", "err", err, "latency", time.Since(start))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	slog.Info("handler: POST /image completed", "content_type", result.ContentType, "bytes", len(result.ImageData), "latency", time.Since(start))
	return c.Blob(http.StatusOK, result.ContentType, result.ImageData)
}
