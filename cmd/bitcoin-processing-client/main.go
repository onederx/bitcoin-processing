package main

import (
	"github.com/onederx/bitcoin-processing/settings"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"os"
	"strings"
)

var apiURLArg string
var apiURL string

var cli = &cobra.Command{
	Use:   "bitcoin-processing-client",
	Short: "CLI client for bitcoin-processing (gateway for accepting and sending bitcoin payments)",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		apiURL = viper.GetString("api.http.address")
		if !strings.HasPrefix(apiURL, "http") {
			apiURL = "http://" + apiURL
		}
	},
}

func main() {
	cobra.OnInitialize(func() {
		viper.BindPFlag("api.http.address", cli.PersistentFlags().Lookup("api-url"))

		if err := settings.LocateAndReadConfigFile(); err == nil {
			log.Printf(
				"Loaded config file %s, will try to use API address from it "+
					"if not given explicitly",
				viper.ConfigFileUsed(),
			)
		}
	})

	cli.PersistentFlags().StringVarP(&apiURLArg, "api-url", "u", "http://localhost:8000", "url of bitcoin-processing API")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
