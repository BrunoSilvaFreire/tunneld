package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable [name]",
	Short: "Enable a tunnel and start it",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Enable(context.Background(), &pb.EnableRequest{Name: name})
		if err != nil {
			log.Fatalf("could not enable tunnel: %v", err)
		}
		fmt.Printf("Tunnel %q enabled and starting\n", name)
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)
}
