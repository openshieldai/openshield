package lib

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ErrorResponse(c *fiber.Ctx, err error) error {
	if strings.Contains(err.Error(), "404") {
		return c.Status(404).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Model not found",
				"type":    "invalid_request_error",
				"param":   "model",
				"code":    "model_not_found",
			}),
		})
	}
	if strings.Contains(err.Error(), "403") {
		return c.Status(403).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Model not found",
				"type":    "invalid_request_error",
				"param":   "model",
				"code":    "model_not_found",
			}),
		})
	}
	if strings.Contains(err.Error(), "401") {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "invalid_request_error",
				"param":   "Authorization",
				"code":    "invalid_header",
			}),
		})
	}
	if strings.Contains(err.Error(), "500") {
		return c.Status(500).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Internal Server Error",
				"type":    "invalid_request_error",
				"param":   "server",
				"code":    "internal_server_error",
			}),
		})
	}
	return c.Status(500).JSON(fiber.Map{
		"error": interface{}(fiber.Map{
			"message": "Internal Server Error",
			"type":    "invalid_request_error",
			"param":   "server",
			"code":    "internal_server_error",
		}),
	})
}
