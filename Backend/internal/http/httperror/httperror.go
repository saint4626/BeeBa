package httperror

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

type Response struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func Handler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	code := "internal_error"
	message := "Internal server error"

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
		message = fiberErr.Message
		code = codeForStatus(status)
	}

	requestID := requestid.FromContext(c)
	return c.Status(status).JSON(Response{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

func NotFound(c fiber.Ctx) error {
	requestID := requestid.FromContext(c)
	return c.Status(fiber.StatusNotFound).JSON(Response{
		Error: ErrorBody{
			Code:      "route_not_found",
			Message:   "Route not found",
			RequestID: requestID,
		},
	})
}

func codeForStatus(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "bad_request"
	case fiber.StatusUnauthorized:
		return "unauthorized"
	case fiber.StatusForbidden:
		return "forbidden"
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusConflict:
		return "conflict"
	case fiber.StatusTooManyRequests:
		return "rate_limited"
	case fiber.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case fiber.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal_error"
	}
}
