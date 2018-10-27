package config

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var cli = &cobra.Command{
	Use:   "bitcoin-processing",
	Short: "Gateway for accepting and sending bitcoin payments",
}

func ReadSettingsAndRun(funcToRun func()) {
	cli.Run = func(cmd *cobra.Command, args []string) {
		funcToRun()
	}

	cobra.OnInitialize(initConfig)

	cli.Flags().StringVarP(&cfgFile, "config-file", "c", "", "config file (default is ./bitcoin-processing.yaml)")
	cli.Flags().StringP("tx-callback", "t", "", "callback url for tx events")
	cli.Flags().StringP("http-host", "H", "", "host for HTTP API to listen on")
	cli.Flags().IntP("http-port", "P", 8000, "port for HTTP API to listen on")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
