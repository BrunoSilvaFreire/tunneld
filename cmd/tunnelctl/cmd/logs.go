package cmd

import (
	"context"
	"fmt"
	"io"
	"log"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
)

var (
	follow bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [name]",
	Short: "Get logs of a tunnel",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getClient()
		defer conn.Close()

		name := args[0]
		stream, err := client.Logs(context.Background(), &pb.LogsRequest{
			Name:   name,
			Follow: follow,
		})
		if err != nil {
			log.Fatalf("could not get logs: %v", err)
		}

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("error receiving logs: %v", err)
			}
			fmt.Print(resp.Line)
		}
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
}
