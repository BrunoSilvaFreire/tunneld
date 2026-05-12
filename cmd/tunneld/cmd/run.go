package cmd

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/BrunoSilvaFreire/tunneld/internal/api"
	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/daemon"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the tunneld supervisor daemon",
	Run: func(cmd *cobra.Command, args []string) {
		run(viper.GetString("config"), viper.GetString("socket"), viper.GetString("key-dir"))
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func run(path, socketPath, keyDirPath string) {
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file %s not found, starting with no tunnels", path)
			cfg = &config.Config{Tunnels: make(map[string]config.TunnelConfig)}
		} else {
			log.Fatalf("Error loading config: %v", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	supervisor := daemon.NewSupervisor(planner, cfg, keyDirPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Remove existing socket if any
	_ = os.Remove(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0660); err != nil {
		log.Printf("Warning: failed to set socket permissions: %v", err)
	}
	defer os.Remove(socketPath)

	s := grpc.NewServer()
	pb.RegisterTunnelServiceServer(s, api.NewTunnelServer(supervisor))
	pb.RegisterKeyServiceServer(s, api.NewKeyServer(supervisor))

	go func() {
		log.Printf("gRPC server listening on %s", socketPath)
		if err := s.Serve(lis); err != nil {
			log.Printf("gRPC server failed: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		log.Println("Stopping gRPC server...")
		s.GracefulStop()
	}()

	log.Println("Starting tunneld supervisor...")
	if err := supervisor.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Supervisor failed: %v", err)
	}
}
