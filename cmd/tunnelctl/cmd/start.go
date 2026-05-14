package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a tunnel",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: tunnelNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Start(context.Background(), &pb.StartRequest{Name: name})
		if err != nil {
			log.Fatalf("could not start: %v", err)
		}
		fmt.Printf("Tunnel %q start signal sent\n", name)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
