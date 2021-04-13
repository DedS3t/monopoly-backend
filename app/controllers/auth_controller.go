package controllers

import (
	"github.com/DedS3t/monopoly-backend/platform/database"
	jwt "github.com/form3tech-oss/jwt-go"
	_ "github.com/go-pg/pg/v10"
	_ "github.com/go-pg/pg/v10/orm"
	"github.com/gofiber/fiber/v2"
	uuid "github.com/satori/go.uuid"
)

type User struct {
	Id       string
	Email    string
	Password string
}

type UserDto struct {
	Email string `json:"email"`
	Pass  string `json:"pass"`
}

func encrypt(pass string) string {
	return pass
}

func Create(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	userDto := new(UserDto)
	if err := c.BodyParser(userDto); err != nil {
		return err
	}

	uuid := uuid.NewV4()
	_, err := db.Model(&User{
		Id:       uuid.String(),
		Email:    userDto.Email,
		Password: encrypt(userDto.Pass)}).Insert()

	if err != nil {
		panic(err)
	}
	return c.Status(201).SendString("Success")
}

func Login(c *fiber.Ctx) error {
	db := database.PostgreSQLConnection()
	defer db.Close()

	userDto := new(UserDto)
	if err := c.BodyParser(userDto); err != nil {
		return err
	}

	user := new(User)
	err := db.Model(user).Where("email = ? AND password = ?", userDto.Email, userDto.Pass).Select()

	if err != nil {
		c.Status(401).SendString("Unauthorized")
	}
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.Id
	t, err := token.SignedString([]byte("secret"))
	if err != nil {
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
