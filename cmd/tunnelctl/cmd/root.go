package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	socketPath string
)

var rootCmd = &cobra.Command{
	Use:   "tunnelctl",
	Short: "tunnelctl is a client to manage tunneld",
	Long:  `A command line tool to interact with the tunneld supervisor daemon.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", "/tmp/tunneld.sock", "Path to tunneld gRPC unix socket")
	viper.BindPFlag("socket", rootCmd.PersistentFlags().Lookup("socket"))
}

func initConfig() {
	viper.AutomaticEnv()
}

func getClient() (pb.TunnelServiceClient, *grpc.ClientConn) {
	s := viper.GetString("socket")
	conn, err := grpc.Dial("unix://"+s,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", s)
		}))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return pb.NewTunnelServiceClient(conn), conn
}
