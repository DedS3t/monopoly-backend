package main

import (
	"github.com/DedS3t/monopoly-backend/app/controllers"
	"github.com/DedS3t/monopoly-backend/pkg/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	jwtware "github.com/gofiber/jwt/v2"
)

func main() {
	app := fiber.New()
	app.Use(cors.New())
	routes.AuthRoutes(app)

	app.Use(jwtware.New(jwtware.Config{
		SigningKey: []byte("secret"),
	}))

	app.Get("/user/cur", controllers.Cur)

	app.Listen(":3000")
}
