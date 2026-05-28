package ratelimit

import (
	"context"
	"fmt"
	"time"

	"beeba.org/internal/ratelimit"

	"github.com/gofiber/fiber/v3"
)

func FixedWindow(limiter ratelimit.Limiter, scope string, limit int64, window time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		if limiter == nil {
			return c.Next()
		}
		key := fmt.Sprintf("rl:%s:%s", scope, c.IP())
		ctx, cancel := context.WithTimeout(c.Context(), time.Second)
		defer cancel()

		ok, err := limiter.Allow(ctx, key, limit, window)
		if err != nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "rate limiter unavailable")
		}
		if !ok {
			return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
		}
		return c.Next()
	}
}
