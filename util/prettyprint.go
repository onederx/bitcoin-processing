package util

import (
	"encoding/json"
	"log"
)

// PrettyPrint attempts to serialize given value as JSON with indentation and
// print the resulting value. Failure to serizlize value to JSON leads to panic.
// This function in intended for debugging
func PrettyPrint(obj interface{}) {
	pretty, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		panic(err)
	}
	log.Print(string(pretty))
}
