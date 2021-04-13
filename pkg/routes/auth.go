package routes

import (
	"github.com/DedS3t/monopoly-backend/app/controllers"
	"github.com/gofiber/fiber/v2"
)

func AuthRoutes(a *fiber.App){
	route := a.Group("/user")

	route.Post("login", controllers.login)
}