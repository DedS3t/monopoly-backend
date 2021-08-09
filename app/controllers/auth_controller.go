package controllers

import (
	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/DedS3t/monopoly-backend/platform/database"
	"github.com/DedS3t/monopoly-backend/platform/logging"
	jwt "github.com/form3tech-oss/jwt-go"
	_ "github.com/go-pg/pg/v10"
	_ "github.com/go-pg/pg/v10/orm"
	"github.com/gofiber/fiber/v2"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

func encrypt(pass string) string {

	bytes := []byte(pass)
	hash, err := bcrypt.GenerateFromPassword(bytes, bcrypt.MinCost)
	if err != nil {
		logging.Error(err.Error())
		panic(err)
	}
	return string(hash)
}

func CreateUser(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	userDto := new(models.UserDto)
	if err := c.BodyParser(userDto); err != nil {
		logging.Error(err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if len(userDto.Email) > 60 {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	uuid := uuid.NewV4()
	_, err := db.Model(&models.User{
		Id:       uuid.String(),
		Email:    userDto.Email,
		Password: encrypt(userDto.Pass)}).Insert()

	if err != nil {
		logging.Error(err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(201)
}

func Login(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	userDto := new(models.UserDto)
	if err := c.BodyParser(userDto); err != nil {
		logging.Error(err.Error())
		return err
	}

	user := new(models.User)
	err := db.Model(user).Where("email = ?", userDto.Email).Limit(1).Select()
	if err != nil {
		logging.Error(err.Error())
		return c.SendStatus(401)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userDto.Pass))
	if err != nil {
		logging.Error(err.Error())
		return c.SendStatus(401)
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.Id
	t, err := token.SignedString([]byte("secret"))
	if err != nil {
		logging.Error(err.Error())
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"access_token": t})
}

func Cur(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	user_id := claims["user_id"].(string)
	return c.SendString(user_id)
}
