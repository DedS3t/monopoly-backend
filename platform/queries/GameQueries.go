package queries

import (
	"github.com/DedS3t/monopoly-backend/app/models"
	"github.com/go-pg/pg/v10"
)

func VerifyGame(id string, db *pg.DB) bool {
	game := &models.Game{Id: id}
	err := db.Model(game).WherePK().Select()
	if err != nil {
		return false
	} else {
		return true
	}
}

// TODO check conccurency

func CreatePlayer(player models.Player, db *pg.DB) error {
	_, err := db.Model(&player).Insert()
	return err
}

func DeletePlayer(user_id string, game string, db *pg.DB) error {

	player := new(models.Player)
	_, err := db.Model(player).Where("user_id = ? and game_id = ?", user_id, game).Delete()
	CheckDB(game, db)
	return err
}

func CheckDB(game_id string, db *pg.DB) {
	var players []models.Player
	err := db.Model(&players).Where("game_id = ?", game_id).Select()
	if err != nil || len(players) == 0 {
		// means there are 0 rows returned

		game := new(models.Game)
		_, err = db.Model(game).Where("id = ?", game_id).Delete()
	}
}
