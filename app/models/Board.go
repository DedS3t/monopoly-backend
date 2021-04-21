package models

type Property struct {
	Name      string `json:"name"`
	Id        string `json:"id"`
	Posistion int    `json:"posistion"`
	Price     int    `json:"price"`
	Rent      int    `json:"rent"`
}
