package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "List all tunnels or status of one",
	ValidArgsFunction: tunnelNameCompletion,
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

		if name != "" && len(resp.Tunnels) == 1 {
			t := resp.Tunnels[0]
			fmt.Printf("Tunnel: %s\n", t.Name)
			fmt.Printf("Status: %s\n", t.Status)
			if t.Error != "" {
				fmt.Printf("Error:  %s\n", t.Error)
			}
			fmt.Println("Spec:")
			m := protojson.MarshalOptions{Multiline: true, Indent: "  "}
			b, _ := m.Marshal(t.Spec)
			fmt.Println(string(b))
			return
		}

		fmt.Printf("%-20s %-10s %-10s %-25s %-20s %s\n", "NAME", "TYPE", "STATUS", "PORTS", "ERROR", "DEPENDENCIES")
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
			var portParts []string
			for _, f := range t.ResolvedForwards {
				part := fmt.Sprintf("%s:%d", f.LocalAddress, f.ActualPort)
				if f.ConfiguredPort == 0 {
					part += "*"
				}
				portParts = append(portParts, part)
			}
			portsStr := strings.Join(portParts, ",")
			fmt.Printf("%-20s %-10s %-10s %-25s %-20s %v\n", t.Name, typeStr, t.Status, portsStr, t.Error, deps)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
