package cmd

import (
	"context"
	"fmt"
	"os"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage keys in tunneld",
}

var keyAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add a key to tunneld",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		path := args[1]

		content, err := os.ReadFile(path)
		if err != nil {
			FatalError("failed to read key file", err)
		}

		_, conn := getClient()
		defer conn.Close()

		keyClient := pb.NewKeyServiceClient(conn)
		_, err = keyClient.AddKey(context.Background(), &pb.AddKeyRequest{
			Name:    name,
			Content: content,
		})
		if err != nil {
			FatalError("failed to add key", err)
		}
		PrintOutput(ActionResponse{Status: "success", Message: fmt.Sprintf("Key %q added successfully", name)}, func() {
			fmt.Printf("Key %q added successfully\n", name)
		})
	},
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List keys in tunneld",
	Run: func(cmd *cobra.Command, args []string) {
		_, conn := getClient()
		defer conn.Close()

		keyClient := pb.NewKeyServiceClient(conn)
		resp, err := keyClient.ListKeys(context.Background(), &pb.ListKeysRequest{})
		if err != nil {
			FatalError("failed to list keys", err)
		}

		PrintOutput(resp, func() {
			fmt.Println("Managed Keys:")
			for _, name := range resp.Names {
				fmt.Printf("- %s\n", name)
			}
		})
	},
}

var keyDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a key from tunneld",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: keyNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		_, conn := getClient()
		defer conn.Close()

		keyClient := pb.NewKeyServiceClient(conn)
		_, err := keyClient.DeleteKey(context.Background(), &pb.DeleteKeyRequest{
			Name: name,
		})
		if err != nil {
			FatalError("failed to delete key", err)
		}
		PrintOutput(ActionResponse{Status: "success", Message: fmt.Sprintf("Key %q deleted successfully", name)}, func() {
			fmt.Printf("Key %q deleted successfully\n", name)
		})
	},
}

func init() {
	keyCmd.AddCommand(keyAddCmd)
	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyDeleteCmd)
	rootCmd.AddCommand(keyCmd)
}
