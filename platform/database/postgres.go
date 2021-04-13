package database

import (
	"os"
	"github.com/go-pg/pg/v10"
	_"github.com/go-pg/pg/v10/orm"
	_ "github.com/joho/godotenv/autoload"
)



func PostgreSQLConnection() *pg.DB{
	return pg.Connect(&pg.Options{
        User: os.Getenv("DB_USER"),
		Addr: os.Getenv("DB_ADDR"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
    })
}