package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a tunnel (no autostart)",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: tunnelNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {

		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Disable(context.Background(), &pb.DisableRequest{Name: name})
		if err != nil {
			log.Fatalf("could not disable tunnel: %v", err)
		}
		fmt.Printf("Tunnel %q disabled and stopping\n", name)
	},
}

func init() {
	rootCmd.AddCommand(disableCmd)
}
