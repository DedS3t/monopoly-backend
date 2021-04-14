package models

type User struct {
	Id       string
	Email    string
	Password string
}

type UserDto struct {
	Email string `json:"email"`
	Pass  string `json:"pass"`
}
