package util

import (
	"encoding/json"
	"fmt"
)

// PrettyPrint attempts to serialize given value as JSON with indentation and
// print the resulting value. In case serialization to JSON fails, corresponding
// error isreturned and nothing is printed.
func PrettyPrint(obj interface{}) error {
	pretty, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

// MustPrettyPrint is a version of PrettyPrint that panics in case of error.
func MustPrettyPrint(obj interface{}) {
	if err := PrettyPrint(obj); err != nil {
		panic(err)
	}
}
