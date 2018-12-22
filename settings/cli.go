package settings

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

// ReadSettingsAndRun reads settings processing both command-line options and
// configuration file and then call given func funcToRun. It should be used by
// entry point of whatever program uses settings, any code that uses settings
// should be called from funcToRun
func ReadSettingsAndRun(funcToRun func(s Settings)) {
	var s Settings
	var cfgFile string

	cli := &cobra.Command{
		Use:   "bitcoin-processing",
		Short: "Gateway for accepting and sending bitcoin payments",
	}

	cli.Run = func(cmd *cobra.Command, args []string) {
		funcToRun(s)
	}

	cobra.OnInitialize(func() {
		var err error
		s, err = NewSettings(cfgFile, cli)
		if err != nil {
			log.Fatalf("Can't read config %s", err)
		}
	})

	cli.Flags().StringVarP(&cfgFile, "config-file", "c", "", "config file (default is ./bitcoin-processing.yaml)")
	cli.Flags().StringP("transaction-callback-url", "t", "", "callback url for tx events")
	cli.Flags().StringP("http-address", "a", "", "host for HTTP API to listen on")
	cli.Flags().StringP("storage-type", "s", "", "type of storage to use")

	if err := cli.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
