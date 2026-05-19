package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var loadPersistent bool

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
			FatalError("failed to load config", err)
		}

		specs, err := cfg.ToSpecs()
		if err != nil {
			FatalError("failed to parse specs", err)
		}

		order, err := dependency.NewPlanner(specs).Plan()
		if err != nil {
			FatalError("failed to resolve dependency order", err)
		}

		hasError := false
		var results []ActionResponse
		for _, spec := range order {
			name := spec.Name()
			protoSpec := spec.ToProto()
			inlineKeys, err := slurpKeys(protoSpec)
			if err != nil {
				msg := fmt.Sprintf("Failed to slurp keys for tunnel %q: %v", name, err)
				log.Print(msg)
				results = append(results, ActionResponse{Status: "error", Message: msg})
				hasError = true
				continue
			}

			_, err = client.Create(context.Background(), &pb.CreateRequest{
				Spec:       protoSpec,
				InlineKeys: inlineKeys,
				Persistent: loadPersistent,
			})
			if err != nil {
				msg := fmt.Sprintf("Failed to create tunnel %q: %v", name, err)
				log.Print(msg)
				results = append(results, ActionResponse{Status: "error", Message: msg})
				hasError = true
				continue
			}

			_, err = client.Start(context.Background(), &pb.StartRequest{Name: name})
			if err != nil {
				msg := fmt.Sprintf("Failed to start tunnel %q: %v", name, err)
				log.Print(msg)
				results = append(results, ActionResponse{Status: "error", Message: msg})
				hasError = true
				continue
			}
			results = append(results, ActionResponse{Status: "success", Message: fmt.Sprintf("Tunnel %q loaded and started (persistent: %v)", name, loadPersistent)})
			if outputFormat == "text" || outputFormat == "" {
				fmt.Printf("Tunnel %q loaded and started (persistent: %v)\n", name, loadPersistent)
			}
		}

		if outputFormat != "text" && outputFormat != "" {
			PrintOutput(results, nil)
		}

		if hasError {
			os.Exit(1)
		}
	},
}

func init() {
	loadCmd.Flags().BoolVar(&loadPersistent, "persistent", false, "Whether the tunnels should be persisted to disk")
	rootCmd.AddCommand(loadCmd)
}
