package settings

import (
	"github.com/spf13/viper"
	"log"
	"net/url"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

var cfgFile string

func locateAndReadConfigFile() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory
		viper.AddConfigPath(".")
		viper.SetConfigName("bitcoin-processing")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("Can't read config:", err)
	}
}

func initConfig() {
	locateAndReadConfigFile()

	// let CLI args override config params
	viper.BindPFlag("transaction.callback", cli.Flags().Lookup("transaction-callback"))
	viper.BindPFlag("api.http.address", cli.Flags().Lookup("http-address"))
	viper.BindPFlag("storage.type", cli.Flags().Lookup("storage-type"))

	// defaults
	viper.SetDefault("http.address", "127.0.0.1:8000")
	viper.SetDefault("bitcoin.node.tls", false)
	viper.SetDefault("bitcoin.poll-interval", 3000)
	viper.SetDefault("transaction.max-confirmations", 6)
	viper.SetDefault("wallet.min-withdraw", 600)
	viper.SetDefault("wallet.min-fee.per-kb", bitcoin.MinimalFeeRate)
	viper.SetDefault("wallet.min-fee.fixed", 500)
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
}

func GetInt64(key string) int64 {
	return viper.GetInt64(key)
}

func GetBool(key string) bool {
	return viper.GetBool(key)
}

func GetURL(key string) string {
	urlValue := viper.GetString(key)
	if _, err := url.ParseRequestURI(urlValue); err != nil {
		log.Fatalf(
			"%s should be set to a valid URL. URL %s",
			key,
			err,
		)
	}
	return urlValue
}

func GetStringMandatory(key string) string {
	value := viper.GetString(key)

	if value == "" {
		log.Fatalf("Error: setting %s is required", key)
	}
	return value
}
