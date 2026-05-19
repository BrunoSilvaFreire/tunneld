package cmd

import (
	"context"
	"fmt"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a tunnel (autostart)",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: tunnelNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {

		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Enable(context.Background(), &pb.EnableRequest{Name: name})
		if err != nil {
			FatalError("could not enable", err)
		}
		PrintOutput(ActionResponse{Status: "success", Message: fmt.Sprintf("Tunnel %q enabled and starting", name)}, func() {
			fmt.Printf("Tunnel %q enabled and starting\n", name)
		})
		},
		}


func init() {
	rootCmd.AddCommand(enableCmd)
}
