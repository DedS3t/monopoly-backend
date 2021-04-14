package controllers

import (
	"fmt"

	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/pkg"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/gofiber/fiber/v2"
)

func CreateGame(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	gameCreateDto := new(models.GameCreateDto)
	if err := c.BodyParser(gameCreateDto); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	game := &models.Game{
		Id:      pkg.RandString(8),
		Name:    gameCreateDto.Name,
		Started: "false",
	}

	_, err := db.Model(game).Insert()
	if err != nil {
		fmt.Println(err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"id": game.Id})
}
