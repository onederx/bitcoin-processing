package settings

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var cli = &cobra.Command{
	Use:   "bitcoin-processing",
	Short: "Gateway for accepting and sending bitcoin payments",
}

// ReadSettingsAndRun reads settings processing both command-line options and
// configuration file and then call given func funcToRun. It should be used by
// entry point of whatever program uses settings, any code that uses settings
// should be called from funcToRun
func ReadSettingsAndRun(funcToRun func()) {
	cli.Run = func(cmd *cobra.Command, args []string) {
		funcToRun()
	}

	cobra.OnInitialize(initConfig)

	cli.Flags().StringVarP(&cfgFile, "config-file", "c", "", "config file (default is ./bitcoin-processing.yaml)")
	cli.Flags().StringP("transaction-callback-url", "t", "", "callback url for tx events")
	cli.Flags().StringP("http-address", "a", "", "host for HTTP API to listen on")
	cli.Flags().StringP("storage-type", "s", "", "type of storage to use")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
