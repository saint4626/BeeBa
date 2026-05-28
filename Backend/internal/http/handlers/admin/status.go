package admin

import (
	"beeba.org/internal/http/middleware/authz"

	"github.com/gofiber/fiber/v3"
)

type StatusHandler struct{}

func NewStatusHandler() StatusHandler {
	return StatusHandler{}
}

func (h StatusHandler) Status(c fiber.Ctx) error {
	session, ok := authz.CurrentSession(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing authenticated session")
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"status": "ok",
			"actor":  session.User,
			"roles":  session.Roles,
		},
	})
}
