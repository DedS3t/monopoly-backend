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
		Id:     pkg.RandString(8),
		Name:   gameCreateDto.Name,
		Status: "false",
	}

	_, err := db.Model(game).Insert()
	if err != nil {
		fmt.Println(err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"id": game.Id})
}

func GetAllAvailGames(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	var games []models.Game
	err := db.Model(&games).Where("status = ?", "false").Select()
	if err != nil {
		panic(err)
	}

	return c.JSON(games)

}

func VerifyGame(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	verifyGameDto := new(models.VerifyGameDto)
	if err := c.QueryParser(verifyGameDto); err != nil {
		fmt.Println(err)
		return err
	}

	game := &models.Game{Id: verifyGameDto.Code}

	err := db.Model(game).WherePK().Select()
	if err != nil {
		fmt.Println(err)
		return c.JSON(fiber.Map{"status": false})
	}
	return c.JSON(fiber.Map{"status": true})

}
