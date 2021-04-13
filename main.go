package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/DedS3t/monopoly-backend/pkg/routes"
)
func main() {
	app := fiber.New()

	routes.AuthRoutes(app)


	app.Listen(":3000")
}