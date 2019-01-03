package main

import (
	"log"

	"github.com/onederx/bitcoin-processing/util"
)

func showResponse(responseData interface{}, respErr error) {
	if respErr != nil {
		log.Fatal(respErr)
	}
	err := util.PrettyPrint(responseData)

	if err != nil {
		log.Fatalf("Failed to marshal API response data as JSON with "+
			"indentation. Error: %s. Response data %v", err, responseData)
	}
}
