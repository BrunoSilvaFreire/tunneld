package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	createConfigPath string
	createPersistent bool
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a tunnel from YAML config",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		if createConfigPath == "" {
			FatalError("--config required", nil)
		}

		data, err := os.ReadFile(createConfigPath)
		if err != nil {
			FatalError("failed to read config", err)
		}

		var tc config.TunnelConfig
		if err := yaml.Unmarshal(data, &tc); err != nil {
			FatalError("failed to parse yaml", err)
		}

		protoSpec, err := tc.ToProto(name)
		if err != nil {
			FatalError("failed to convert to proto", err)
		}

		inlineKeys, err := slurpKeys(protoSpec)
		if err != nil {
			FatalError("failed to slurp keys", err)
		}

		_, err = client.Create(context.Background(), &pb.CreateRequest{
			Spec:       protoSpec,
			InlineKeys: inlineKeys,
			Persistent: createPersistent,
		})
		if err != nil {
			FatalError("could not create", err)
		}
		PrintOutput(ActionResponse{Status: "success", Message: fmt.Sprintf("Tunnel %q created (persistent: %v)", name, createPersistent)}, func() {
			fmt.Printf("Tunnel %q created (persistent: %v)\n", name, createPersistent)
		})
	},
}

func init() {
	createCmd.Flags().StringVar(&createConfigPath, "config", "", "Path to tunnel YAML config")
	createCmd.Flags().BoolVar(&createPersistent, "persistent", false, "Whether the tunnel should be persisted to disk")
	createCmd.MarkFlagRequired("config")
	rootCmd.AddCommand(createCmd)
}
