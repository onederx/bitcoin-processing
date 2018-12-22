package settings

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onederx/bitcoin-processing/bitcoin"
)

// Settings is responsible for gettings all settings used by processing app.
// Settings can come from command line or from configuration file. Parts of
// processing app get them by calling Get* methods
type Settings interface {
	GetString(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetBool(key string) bool
	GetURL(key string) string
	GetStringMandatory(key string) string
	GetBTCAmount(key string) bitcoin.BTCAmount
	ConfigFileUsed() string
	GetViper() *viper.Viper
}

type settings struct {
	cfgFile string
	viper   *viper.Viper
}

// NewSettings creates new Settings instance. Settings come from command line
// and from config file, so this function accepts a path to config file and
// a pointer to root cobra.Command.
// In case given path to config file is an empty string, config will be auto -
// searched: currently, directories "/etc/bitcoin-processing" and current
// working directory will be checked for a file names config.{yml,json,...}.
// (possible extensions are ones supported by viper)
func NewSettings(cfgFile string, cli *cobra.Command) (Settings, error) {
	s := &settings{cfgFile: cfgFile, viper: viper.New()}
	if cfgFile != "" {
		// Use config file from the flag.
		s.viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory and /etc/bitcoin-processing
		s.viper.AddConfigPath("/etc/bitcoin-processing")
		s.viper.AddConfigPath(".")
		s.viper.SetConfigName("config")
	}

	err := s.viper.ReadInConfig()

	if err != nil {
		return s, err
	}
	s.initConfig(cli)
	return s, nil
}
