package models

type Property struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Group           string `json:"group"`
	Posistion       int    `json:"posistion"`
	Price           int    `json:"price"`
	Rent            int    `json:"rent"`
	Mulriplied_Rent []int  `json:"multiplied_rent"`
	Mortgage        int    `json:"mortgage"`
	HouseCost       int    `json:"housecost"`
	Action          string `json:"action"`
}

type Special struct {
	Info    string `json:"info"`
	Action  string `json:"action"` // "change" - balance update, "move" - move spaces, "other" - other actions
	Payload int    `json:"payload"`
}
