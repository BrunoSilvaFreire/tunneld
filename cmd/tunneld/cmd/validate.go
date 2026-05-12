package cmd

import (
	"fmt"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the tunnel configuration",
	Run: func(cmd *cobra.Command, args []string) {
		validate(viper.GetString("config"))
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func validate(path string) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		os.Exit(1)
	}

	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	if _, err := planner.Plan(); err != nil {
		fmt.Printf("Dependency validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Config is valid")
}
