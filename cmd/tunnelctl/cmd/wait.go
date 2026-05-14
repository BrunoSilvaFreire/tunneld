package cmd

import (
	"context"
	"fmt"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var waitTimeout int

var waitCmd = &cobra.Command{
	Use:   "wait <name>",
	Short: "Wait for a tunnel to be ready",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: tunnelNameCompletion,
	Run: func(cmd *cobra.Command, args []string) {

		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		fmt.Printf("Waiting for tunnel %q (timeout %ds)...\n", name, waitTimeout)
		resp, err := client.Wait(context.Background(), &pb.WaitRequest{
			Name:           name,
			TimeoutSeconds: int64(waitTimeout),
		})
		if err != nil {
			log.Fatalf("wait failed: %v", err)
		}
		fmt.Printf("Tunnel %q is %s\n", name, resp.Status)
		for _, f := range resp.ResolvedForwards {
			suffix := ""
			if f.ConfiguredPort == 0 {
				suffix = " (dynamic)"
			}
			fmt.Printf("  %s:%d → :%d%s\n", f.LocalAddress, f.ActualPort, f.RemotePort, suffix)
		}
	},
}

func init() {
	waitCmd.Flags().IntVar(&waitTimeout, "timeout", 30, "Timeout in seconds")
	rootCmd.AddCommand(waitCmd)
}
