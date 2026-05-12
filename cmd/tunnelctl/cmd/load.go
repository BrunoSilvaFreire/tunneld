package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load <path>",
	Short: "Load all tunnels from a YAML config file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		configPath := args[0]
		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		specs, err := cfg.ToSpecs()
		if err != nil {
			log.Fatalf("failed to parse specs: %v", err)
		}

		for name, spec := range specs {
			_, err := client.Create(context.Background(), &pb.CreateRequest{
				Spec: spec.ToProto(),
			})
			if err != nil {
				fmt.Printf("Failed to create tunnel %q: %v\n", name, err)
				continue
			}

			_, err = client.Start(context.Background(), &pb.StartRequest{Name: name})
			if err != nil {
				fmt.Printf("Failed to start tunnel %q: %v\n", name, err)
				continue
			}
			fmt.Printf("Tunnel %q loaded and started\n", name)
		}
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}
