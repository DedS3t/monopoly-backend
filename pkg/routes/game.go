package routes

import (
	"github.com/DedS3t/monopoly-backend/app/controllers"
	"github.com/gofiber/fiber/v2"
)

func GameRoutes(a *fiber.App) {
	route := a.Group("/game")
	route.Post("create", controllers.CreateGame)
	route.Get("/verify", controllers.VerifyGame)
	route.Get("/all", controllers.GetAllAvailGames)
	route.Get("/find", controllers.FindAvailGame)
}
