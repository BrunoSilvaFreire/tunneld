package main

import (
	"context"
	"flag"
	"fmt"
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

	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "tunnels.yaml", "Path to config file")
	socketPath := flag.String("socket", "/tmp/tunneld.sock", "Path to gRPC unix socket")
	flag.Parse()

	if flag.NArg() > 0 {
		cmd := flag.Arg(0)
		switch cmd {
		case "validate":
			validate(*configPath)
			return
		case "plan":
			plan(*configPath)
			return
		case "run":
			// continue
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			os.Exit(1)
		}
	}

	run(*configPath, *socketPath)
}

func validate(path string) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		os.Exit(1)
	}

	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	if _, err := planner.Plan(); err != nil {
		fmt.Printf("Dependency validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Config is valid")
}

func plan(path string) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	order, err := planner.Plan()
	if err != nil {
		fmt.Printf("Error planning: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Startup order:")
	for i, spec := range order {
		fmt.Printf("%d. %s (%s)\n", i+1, spec.Name(), spec.Type())
		if len(spec.Dependencies()) > 0 {
			fmt.Printf("   Depends on: %v\n", spec.Dependencies())
		}
	}
}

func run(path, socketPath string) {
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	specs, _ := cfg.ToSpecs()
	planner := dependency.NewPlanner(specs)
	supervisor := daemon.NewSupervisor(planner)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ensure socket directory exists
	// (socketPath is /tmp/tunneld.sock usually, but user can change it)
	// We don't necessarily want to mkdir /tmp, but if it's /run/tunneld/tunneld.sock we might.

	// Remove existing socket if any
	_ = os.Remove(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", socketPath, err)
	}
	defer os.Remove(socketPath)

	s := grpc.NewServer()
	pb.RegisterTunnelServiceServer(s, api.NewTunnelServer(supervisor))

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
