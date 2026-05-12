package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "List all tunnels or status of one",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		resp, err := client.Status(context.Background(), &pb.StatusRequest{Name: name})
		if err != nil {
			log.Fatalf("could not get status: %v", err)
		}
		fmt.Printf("%-20s %-10s %-10s %s\n", "NAME", "TYPE", "STATUS", "DEPENDENCIES")
		for _, t := range resp.Tunnels {
			typeStr := "unknown"
			deps := []string{}
			if t.Spec != nil {
				deps = t.Spec.DependsOn
				if t.Spec.GetSsh() != nil {
					typeStr = "ssh"
				} else if t.Spec.GetKubectl() != nil {
					typeStr = "kubectl"
				}
			}
			fmt.Printf("%-20s %-10s %-10s %v\n", t.Name, typeStr, t.Status, deps)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
