package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a tunnel",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: tunnelNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Stop(context.Background(), &pb.StopRequest{Name: name})
		if err != nil {
			log.Fatalf("could not stop: %v", err)
		}
		fmt.Printf("Tunnel %q stop signal sent\n", name)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
