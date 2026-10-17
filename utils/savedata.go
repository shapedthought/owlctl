package utils

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v2"
)

func SaveData[T any](data *T, name string) {
	file, err := yaml.Marshal(&data)
	IsErr(err)

	// os.WriteFile errors include the file path
	IsErr(os.WriteFile(name+".yaml", file, 0644))
}

// SaveJsonWithError writes data as indented JSON to name+".json" and returns
// any marshal or write error instead of terminating the program.
func SaveJsonWithError[T any](data *T, name string) error {
	file, err := json.MarshalIndent(&data, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(name+".json", file, 0644)
}

func SaveJson[T any](data *T, name string) {
	IsErr(SaveJsonWithError(data, name))
}
