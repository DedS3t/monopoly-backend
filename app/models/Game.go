package models

type Game struct {
	Id     string
	Name   string
	Status string
}

type GameCreateDto struct {
	Name string
}

type VerifyGameDto struct {
	Code string
}
