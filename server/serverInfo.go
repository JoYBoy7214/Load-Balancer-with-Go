package server

import (
	"encoding/json"
	"log"
	"os"
)

type ServerInfos struct {
	Port     int    `json:"port"`
	Strategy string `json:"strategy"`
	Backends []struct {
		Url    string `json:"url"`
		Weight int64  `json:"weight"`
	} `json:"backends"`
}

func GetServerInfo(path string) []ServerInfos {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)

	}
	var s []ServerInfos
	err = json.Unmarshal(data, &s)
	if err != nil {
		log.Fatal(err)

	}
	return s

}
