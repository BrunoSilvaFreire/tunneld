package cmd

import (
	"fmt"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show the tunnel startup plan",
	Run: func(cmd *cobra.Command, args []string) {
		plan(viper.GetString("config"))
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func plan(path string) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	order, err := planner.Plan()
	if err != nil {
		fmt.Printf("Error planning: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Startup order:")
	for i, spec := range order {
		fmt.Printf("%d. %s (%s)\n", i+1, spec.Name(), spec.Type())
		if len(spec.Dependencies()) > 0 {
			fmt.Printf("   Depends on: %v\n", spec.Dependencies())
		}
	}
}
