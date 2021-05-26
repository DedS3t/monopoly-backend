package board

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/DedS3t/monopoly-backend/app/models"
)

func LoadProperties() map[string]models.Property {
	var properties map[string]models.Property
	jsonFile, err := os.Open("platform/board//properties.json")
	if err != nil {
		panic(err)
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal([]byte(byteValue), &properties)
	return properties
}

func LoadSpecial() map[string][]models.Special {
	var specials map[string][]models.Special
	jsonFile, err := os.Open("platform/board//special.json")
	if err != nil {
		panic(err)
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal([]byte(byteValue), &specials)
	return specials
}

func GetByPos(pos int, properties *map[string]models.Property) (models.Property, error) { // O(1) time complexity
	if property, found := (*properties)[strconv.Itoa(pos)]; found {
		return property, nil
	} else {
		return models.Property{}, errors.New("not found")
	}
}

func CanBuildHouses(property models.Property) bool {
	return !(property.Type == "special" || property.Group == "railroad" || property.Group == "utility")
}
