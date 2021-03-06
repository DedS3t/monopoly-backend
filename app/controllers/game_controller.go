package controllers

import (
	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/pkg"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/DedS3t/monopoly-backend/platform/logging"
	"github.com/gofiber/fiber/v2"
)

func CreateGame(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	gameCreateDto := new(models.GameCreateDto)
	if err := c.BodyParser(gameCreateDto); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if gameCreateDto.Type != "public" && gameCreateDto.Type != "private" {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if len(gameCreateDto.Name) > 60 {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	game := &models.Game{
		Id:     pkg.RandString(8),
		Name:   gameCreateDto.Name,
		Status: "false",
		Type:   gameCreateDto.Type,
	}

	_, err := db.Model(game).Insert()
	if err != nil {
		logging.Error(err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"id": game.Id})
}

func GetAllAvailGames(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	var games []models.Game
	err := db.Model(&games).Where("status = ? and type = ?", "false", "public").Select()
	if err != nil {
		logging.Error(err.Error())
		panic(err)
	}

	return c.JSON(games)
}

func FindAvailGame(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	var games []models.Game
	err := db.Model(&games).Where("status = ? and type = ?", "false", "public").Limit(1).Select()
	if err != nil {
		logging.Error(err.Error())
		panic(err)
	}
	if len(games) < 1 {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.JSON(games[0])
}

func VerifyGame(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	verifyGameDto := new(models.VerifyGameDto)
	if err := c.QueryParser(verifyGameDto); err != nil {
		logging.Error(err.Error())
		return err
	}

	game := &models.Game{Id: verifyGameDto.Code}

	err := db.Model(game).WherePK().Select()
	// TODO if game.Status != false that means game is in progress... Check if player is part of game
	if err != nil {
		logging.Error(err.Error())
		return c.JSON(fiber.Map{"status": false})
	}

	if game.Status == "in progress" {
		// in progress check if player is part of game

		player := &models.Player{}

		err = db.Model(player).Where("user_id = ? and game_id = ?", verifyGameDto.User_id, verifyGameDto.Code).Select()

		if err != nil {
			// not part of game
			return c.JSON(fiber.Map{"status": false})
		} else {
			return c.JSON(fiber.Map{"status": "rejoin"})
		}
	}

	return c.JSON(fiber.Map{"status": true})

}
