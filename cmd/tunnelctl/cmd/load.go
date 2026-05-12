package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

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

		hasError := false
		for name, spec := range specs {
			protoSpec := spec.ToProto()
			inlineKeys, err := slurpKeys(protoSpec)
			if err != nil {
				log.Printf("Failed to slurp keys for tunnel %q: %v", name, err)
				hasError = true
				continue
			}

			_, err = client.Create(context.Background(), &pb.CreateRequest{
				Spec:       protoSpec,
				InlineKeys: inlineKeys,
			})
			if err != nil {
				log.Printf("Failed to create tunnel %q: %v", name, err)
				hasError = true
				continue
			}

			_, err = client.Start(context.Background(), &pb.StartRequest{Name: name})
			if err != nil {
				log.Printf("Failed to start tunnel %q: %v", name, err)
				hasError = true
				continue
			}
			fmt.Printf("Tunnel %q loaded and started\n", name)
		}

		if hasError {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}
