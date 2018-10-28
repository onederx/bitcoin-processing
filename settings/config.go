package settings

import (
	"github.com/spf13/viper"
	"log"
	"net/url"
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
	viper.BindPFlag("tx-callback", cli.Flags().Lookup("tx-callback"))
	viper.BindPFlag("api.http.address", cli.Flags().Lookup("http-address"))
	viper.BindPFlag("storage.type", cli.Flags().Lookup("storage-type"))

	// defaults
	viper.SetDefault("http.address", "127.0.0.1:8000")
	viper.SetDefault("bitcoin.node.tls", false)
	viper.SetDefault("bitcoin.poll-interval", 3000)
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
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
