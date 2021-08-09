package main

import (
	"github.com/DedS3t/monopoly-backend/app/controllers"
	"github.com/DedS3t/monopoly-backend/pkg/routes"
	"github.com/DedS3t/monopoly-backend/platform/logging"
	socket "github.com/DedS3t/monopoly-backend/platform/sockets"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	jwtware "github.com/gofiber/jwt/v2"
)

func main() {
	logging.Init()

	app := fiber.New()

	app.Use(cors.New())
	routes.AuthRoutes(app)
	routes.GameRoutes(app)

	app.Use(jwtware.New(jwtware.Config{
		SigningKey: []byte("secret"),
	}))

	app.Get("/user/cur", controllers.Cur)
	go socket.CreateSocketIOServer()
	app.Listen(":4101")
}
