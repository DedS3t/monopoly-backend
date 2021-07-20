package models

type Game struct {
	Id     string
	Name   string
	Status string
	Type   string
}

type GameCreateDto struct {
	Name string
	Type string
}

type VerifyGameDto struct {
	Code    string
	User_id string
}
