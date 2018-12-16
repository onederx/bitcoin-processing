package settings

import (
	"github.com/spf13/viper"
	"log"
	"net/url"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

var cfgFile string

// LocateAndReadConfigFile does what its name implies - locates and reads config
// file. Its name may either be set by variable cfgFile (that will be populated
// by value given on command line, if any) or located by viper library itself
// by searching in default locations. It returns nil if config file was loaded
// and error with reason otherwise.
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

// GetString takes a string value from config. It simply calls viper.GetString.
// If value is absent or has wrong type, it
func GetString(key string) string {
	return viper.GetString(key)
}

// GetString takes a string value from config. It simply calls viper.GetString
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

// GetBTCAmount takes amount of bitcoins from config and returns it as
// bitcoin.BTCAmount. It reads amount as a string (with viper.GetString) and
// then tries to convert it using bitcoin.BTCAmountFromStringedFloat, any
// error encountered during convertion is fatal. This means that absent value
// or value of incompatible type will lead to fatal error. This also means
// bitcoin amounts can be given in config as strings instead of floats to
// bypass possible loss of precision due to floating-point representation errors
func GetBTCAmount(key string) bitcoin.BTCAmount {
	value, err := bitcoin.BTCAmountFromStringedFloat(viper.GetString(key))
	if err != nil {
		log.Fatalf("Error converting value of setting %s to bitcoin amount: %s",
			key, err)
	}
	return value
}
