package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a tunnel",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		_, err := client.Delete(context.Background(), &pb.DeleteRequest{Name: name})
		if err != nil {
			log.Fatalf("could not delete: %v", err)
		}
		fmt.Printf("Tunnel %q deleted\n", name)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
