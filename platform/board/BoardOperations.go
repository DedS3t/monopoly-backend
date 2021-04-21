package board

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/DedS3t/monopoly-backend/app/models"
)

func LoadProperties() []models.Property {
	var properties []models.Property
	jsonFile, err := os.Open("platform/board//properties.json")
	if err != nil {
		panic(err)
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal([]byte(byteValue), &properties)
	return properties
}

func GetByPos(pos int, properties *[]models.Property) (models.Property, error) { // O(N) time complexity
	for _, property := range *properties {
		if property.Posistion == pos {
			return property, nil
		}
	}
	return models.Property{}, errors.New("not found")

}

func GetById(id string, properties *[]models.Property) (models.Property, error) { // O(N) time complexity
	for _, property := range *properties {
		if property.Id == id {
			return property, nil
		}
	}
	return models.Property{}, errors.New("not found")
}
