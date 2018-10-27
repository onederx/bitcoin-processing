package config

import (
    "os"
    "log"
    "github.com/spf13/viper"
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
        log.Println("Can't read config:", err)
        os.Exit(1)
    }
}

func initConfig() {
    locateAndReadConfigFile()
    
    viper.BindPFlag("tx-callback", cli.Flags().Lookup("tx-callback"))
}

func GetString(key string) string {
    return viper.GetString(key)
}