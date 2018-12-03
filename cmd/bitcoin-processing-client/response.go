package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
)

func showResponse(responseBody io.Reader) {
	var responseData interface{}

	response, err := ioutil.ReadAll(responseBody)

	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(response, &responseData)

	if err != nil {
		log.Fatalf(
			"Failed to unmarshal API response body as JSON with error %s. "+
				"Response body %s",
			err,
			response,
		)
	}

	responseBeautified, err := json.MarshalIndent(responseData, "", "    ")

	if err != nil {
		log.Fatalf(
			"Failed to marshal API response data as JSON with indentation "+
				"Error: %s. Response data %v",
			err,
			responseData,
		)
	}
	fmt.Printf("%s\n", responseBeautified)
}
