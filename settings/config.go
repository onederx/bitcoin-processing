package settings

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"net/url"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

func (s *settings) initConfig(cli *cobra.Command) {
	// let CLI args override config params
	s.viper.BindPFlag("transaction.callback.url", cli.Flags().Lookup("transaction-callback-url"))
	s.viper.BindPFlag("api.http.address", cli.Flags().Lookup("http-address"))
	s.viper.BindPFlag("storage.type", cli.Flags().Lookup("storage-type"))

	// defaults
	s.viper.SetDefault("http.address", "127.0.0.1:8000")
	s.viper.SetDefault("bitcoin.node.tls", false)
	s.viper.SetDefault("bitcoin.poll_interval", 3000)
	s.viper.SetDefault("transaction.max_confirmations", 6)
	s.viper.SetDefault("wallet.min_withdraw", 0.000006)
	s.viper.SetDefault("wallet.min_fee.per_kb", bitcoin.MinimalFeeRateBTC)
	s.viper.SetDefault("wallet.min_fee.fixed", 0.000005)
	s.viper.SetDefault("wallet.min_withdraw_without_manual_confirmation", 0.0)
	s.viper.SetDefault("transaction.callback.backoff", 100)
}

// GetString takes a string value from config. It simply calls viper.GetString.
// If value is absent or has incompatible type (for example array), it will
// return empty string
func (s *settings) GetString(key string) string {
	return s.viper.GetString(key)
}

// GetInt takes int value from config. It simply calls viper.GetInt.
// If value is absent or has incompatible type (for example 'x'), it will
// return 0
func (s *settings) GetInt(key string) int {
	return s.viper.GetInt(key)
}

// GetInt64 takes int64 value from config. It simply calls viper.GetInt64.
// If value is absent or has incompatible type (for example 'x'), it will
// return 0
func (s *settings) GetInt64(key string) int64 {
	return s.viper.GetInt64(key)
}

// GetBool takes boolean value from config. It simply calls viper.GetBool.
// If value is absent or has incompatible type (for example 'nottruefalse'),
// it will return false. It should be noted that compatibility rules are rather
// complex, for example strings '1', 't', 'T', 'TRUE' evaluate to true,
// strings '2', 'tr', 'TRU', 'yes' evaluate to false
func (s *settings) GetBool(key string) bool {
	return s.viper.GetBool(key)
}

// GetURL takes URL value from config. It is a wrapper around viper.GetString
// that calls ParseRequestURI to check that resulting value is a valid url.
// Any error from ParseRequestURI is fatal. As a result, if value in config
// is absent or has incompatible, this function will fail because empty string
// is not a valid URL
func (s *settings) GetURL(key string) string {
	urlValue := s.viper.GetString(key)
	if _, err := url.ParseRequestURI(urlValue); err != nil {
		log.Fatalf(
			"%s should be set to a valid URL. URL %s",
			key,
			err,
		)
	}
	return urlValue
}

// GetStringMandatory takes a string value from config ensuring it is not empty.
// It calls viper.GetString and if that returns an empty string (which will
// happen if value in config IS empty string, or if it is absent or has
// incompatible type) this function will crash.
func (s *settings) GetStringMandatory(key string) string {
	value := s.viper.GetString(key)

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
func (s *settings) GetBTCAmount(key string) bitcoin.BTCAmount {
	value, err := bitcoin.BTCAmountFromStringedFloat(s.viper.GetString(key))
	if err != nil {
		log.Fatalf("Error converting value of setting %s to bitcoin amount: %s",
			key, err)
	}
	return value
}

// ConfigFileUsed returns path to config file currently used. This simply calls
// viper.ConfigFileUsed()
func (s *settings) ConfigFileUsed() string {
	return s.viper.ConfigFileUsed()
}

// ConfigFileUsed returns a pointer to underlying Viper object
func (s *settings) GetViper() *viper.Viper {
	return s.viper
}
