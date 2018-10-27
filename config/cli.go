package config

import (
    "log"
    "os"
    "github.com/spf13/cobra"
)

var txCallback string

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
    cli.Flags().StringVarP(&txCallback, "tx-callback", "t", "", "callback url for tx events")

    if err := cli.Execute(); err != nil {
        log.Println(err)
        os.Exit(1)
    }
}