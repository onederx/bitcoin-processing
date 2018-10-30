package util

import (
	"encoding/json"
	"log"
)

func PrettyPrint(obj interface{}) {
	pretty, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		panic(err)
	}
	log.Print(string(pretty))
}
