package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/constants"
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

	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", constants.DefaultSocketPath, "Path to tunneld gRPC unix socket")
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

func getSilentClient() (pb.TunnelServiceClient, *grpc.ClientConn, error) {
	s := viper.GetString("socket")
	conn, err := grpc.Dial("unix://"+s,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", s)
		}))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewTunnelServiceClient(conn), conn, nil
}

func getSilentKeyClient() (pb.KeyServiceClient, *grpc.ClientConn, error) {
	s := viper.GetString("socket")
	conn, err := grpc.Dial("unix://"+s,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", s)
		}))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewKeyServiceClient(conn), conn, nil
}

func tunnelNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultGRPCRequestTimeout)
	defer cancel()

	client, conn, err := getSilentClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer conn.Close()

	resp, err := client.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, t := range resp.Tunnels {
		names = append(names, t.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func keyNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultGRPCRequestTimeout)
	defer cancel()

	client, conn, err := getSilentKeyClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer conn.Close()

	resp, err := client.ListKeys(ctx, &pb.ListKeysRequest{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return resp.Names, cobra.ShellCompDirectiveNoFileComp
}
