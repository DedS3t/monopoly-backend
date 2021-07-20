package models

type Player struct {
	User_id  string
	Game_id  string
	Username string
	Active   string
}

type PlayerDto struct {
	Username   string
	Balance    int
	Pos        int
	Color      string
	Properties []interface{}
	Jail       bool
}
