package settings

import (
	"github.com/spf13/viper"
	"log"
	"net/url"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

var cfgFile string

func LocateAndReadConfigFile() error {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory and /etc/bitcoin-processing
		viper.AddConfigPath("/etc/bitcoin-processing")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
	}

	return viper.ReadInConfig()
}

func initConfig() {
	if err := LocateAndReadConfigFile(); err != nil {
		log.Fatal("Can't read config:", err)
	}

	// let CLI args override config params
	viper.BindPFlag("transaction.callback.url", cli.Flags().Lookup("transaction-callback-url"))
	viper.BindPFlag("api.http.address", cli.Flags().Lookup("http-address"))
	viper.BindPFlag("storage.type", cli.Flags().Lookup("storage-type"))

	// defaults
	viper.SetDefault("http.address", "127.0.0.1:8000")
	viper.SetDefault("bitcoin.node.tls", false)
	viper.SetDefault("bitcoin.poll_interval", 3000)
	viper.SetDefault("transaction.max_confirmations", 6)
	viper.SetDefault("wallet.min_withdraw", 0.000006)
	viper.SetDefault("wallet.min_fee.per_kb", bitcoin.MinimalFeeRateBTC)
	viper.SetDefault("wallet.min_fee.fixed", 0.000005)
	viper.SetDefault("wallet.min_withdraw_without_manual_confirmation", 0.0)
	viper.SetDefault("transaction.callback.backoff", 100)
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

func GetBTCAmount(key string) bitcoin.BTCAmount {
	return bitcoin.BTCAmountFromFloat(viper.GetFloat64(key))
}
