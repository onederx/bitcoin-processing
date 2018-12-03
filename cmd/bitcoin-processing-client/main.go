package main

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var cli = &cobra.Command{
	Use:   "bitcoin-processing-client",
	Short: "CLI client for bitcoin-processing (gateway for accepting and sending bitcoin payments)",
}

var apiURL string

func main() {
	cli.PersistentFlags().StringVarP(&apiURL, "api-url", "u", "http://localhost:8000", "url of bitcoin-processing API")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
