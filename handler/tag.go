package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/usecase"
)

type TagHandler struct {
	tag usecase.TagUsecase
}

func NewTagHandler(tag usecase.TagUsecase) *TagHandler {
	return &TagHandler{tag: tag}
}

// Tag handles POST /tag — image tagging.
func (h *TagHandler) Tag(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
	}
	if len(body) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "request body is empty"})
	}

	input := usecase.TagInput{
		PipelineName: c.QueryParam("pipeline"),
		Image:        body,
	}
	if mt := c.QueryParam("max_tags"); mt != "" {
		input.MaxTags, _ = strconv.Atoi(mt)
	}

	result, err := h.tag.Run(c.Request().Context(), input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]any{"tags": result.Tags})
}
