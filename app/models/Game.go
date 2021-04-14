package models

type Game struct {
	Id      string
	Name    string
	Started string
}

type GameCreateDto struct {
	Name string
}
