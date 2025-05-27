package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/nazzarr03/go-resume-ai/pkg/utils"
)

func AuthMiddleware(c *fiber.Ctx) error {
	authorization := c.Get("Authorization")

	if !strings.HasPrefix(authorization, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "fail", "message": "Invalid Token"})
	}

	tokenString := strings.TrimPrefix(authorization, "Bearer ")

	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "fail", "message": "You are not logged in"})
	}

	token, err := utils.ValidateJWT(tokenString)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "fail", "message": "Invalid Token"})
	}

	c.Locals("user_id", token.UserId)

	return c.Next()
}