package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	socketPath string
	keyDir     string
)

var rootCmd = &cobra.Command{
	Use:   "tunneld",
	Short: "tunneld is a persistent local tunnel supervisor daemon",
	Long:  `A daemon that manages and monitors SSH and kubectl tunnels based on a YAML configuration.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "tunnels.yaml", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", "/tmp/tunneld.sock", "Path to gRPC unix socket")
	rootCmd.PersistentFlags().StringVar(&keyDir, "key-dir", "/var/lib/tunneld/keys", "Directory to store managed keys")

	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("socket", rootCmd.PersistentFlags().Lookup("socket"))
	viper.BindPFlag("key-dir", rootCmd.PersistentFlags().Lookup("key-dir"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
