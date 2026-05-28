package openapi

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	path string
}

func New() Handler {
	return Handler{path: findSpecPath()}
}

func (h Handler) YAML(c fiber.Ctx) error {
	if h.path == "" {
		return fiber.NewError(fiber.StatusNotFound, "OpenAPI document is not available")
	}

	spec, err := os.ReadFile(h.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fiber.NewError(fiber.StatusNotFound, "OpenAPI document is not available")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read OpenAPI document")
	}

	c.Set(fiber.HeaderContentType, "application/yaml; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.Send(spec)
}

func findSpecPath() string {
	candidates := []string{
		filepath.Join("openapi", "openapi.yaml"),
		filepath.Join("Backend", "openapi", "openapi.yaml"),
		filepath.Join("/app", "openapi", "openapi.yaml"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}
