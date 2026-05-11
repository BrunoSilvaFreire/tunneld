package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

func main() {
	socketPath := flag.String("socket", "/tmp/tunneld.sock", "Path to tunneld gRPC unix socket")
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		os.Exit(1)
	}

	// Connect to gRPC server over Unix Domain Socket
	conn, err := grpc.Dial("unix://"+*socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", *socketPath)
		}))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewTunnelServiceClient(conn)

	cmd := flag.Arg(0)
	switch cmd {
	case "status":
		status(client, flag.Args()[1:])
	case "start":
		start(client, flag.Args()[1:])
	case "stop":
		stop(client, flag.Args()[1:])
	case "wait":
		wait(client, flag.Args()[1:])
	case "create":
		create(client, flag.Args()[1:])
	case "load":
		load(client, flag.Args()[1:])
	case "delete":
		deleteTunnel(client, flag.Args()[1:])
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage: tunnelctl [options] <command> [args]")
	fmt.Println("Options:")
	fmt.Println("  -socket string    Path to tunneld gRPC unix socket (default \"/tmp/tunneld.sock\")")
	fmt.Println("Commands:")
	fmt.Println("  status [name]       List all tunnels or status of one")
	fmt.Println("  start <name>        Start a tunnel")
	fmt.Println("  stop <name>         Stop a tunnel")
	fmt.Println("  wait <name>         Wait for a tunnel to be healthy")
	fmt.Println("  create <name>       Create a tunnel from YAML config")
	fmt.Println("  load <path>         Load all tunnels from a YAML config file")
	fmt.Println("  delete <name>       Delete a tunnel")
}

func status(client pb.TunnelServiceClient, args []string) {
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
}

func start(client pb.TunnelServiceClient, args []string) {
	if len(args) == 0 {
		log.Fatal("tunnel name required")
	}
	_, err := client.Start(context.Background(), &pb.StartRequest{Name: args[0]})
	if err != nil {
		log.Fatalf("could not start: %v", err)
	}
	fmt.Printf("Tunnel %q start signal sent\n", args[0])
}

func stop(client pb.TunnelServiceClient, args []string) {
	if len(args) == 0 {
		log.Fatal("tunnel name required")
	}
	_, err := client.Stop(context.Background(), &pb.StopRequest{Name: args[0]})
	if err != nil {
		log.Fatalf("could not stop: %v", err)
	}
	fmt.Printf("Tunnel %q stop signal sent\n", args[0])
}

func wait(client pb.TunnelServiceClient, args []string) {
	if len(args) == 0 {
		log.Fatal("tunnel name required")
	}

	waitCmd := flag.NewFlagSet("wait", flag.ExitOnError)
	timeoutPtr := waitCmd.Int("timeout", 30, "Timeout in seconds")
	_ = waitCmd.Parse(args[1:])

	fmt.Printf("Waiting for tunnel %q (timeout %ds)...\n", args[0], *timeoutPtr)
	resp, err := client.Wait(context.Background(), &pb.WaitRequest{
		Name:           args[0],
		TimeoutSeconds: int64(*timeoutPtr),
	})
	if err != nil {
		log.Fatalf("wait failed: %v", err)
	}
	fmt.Printf("Tunnel %q is %s\n", args[0], resp.Status)
}

func create(client pb.TunnelServiceClient, args []string) {
	if len(args) < 1 {
		log.Fatal("tunnel name required")
	}
	name := args[0]
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	configPath := createCmd.String("config", "", "Path to tunnel YAML config")
	_ = createCmd.Parse(args[1:])

	if *configPath == "" {
		log.Fatal("--config required")
	}

	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	var tc config.TunnelConfig
	if err := yaml.Unmarshal(data, &tc); err != nil {
		log.Fatalf("failed to parse yaml: %v", err)
	}

	protoSpec, err := tc.ToProto(name)
	if err != nil {
		log.Fatalf("failed to convert to proto: %v", err)
	}

	_, err = client.Create(context.Background(), &pb.CreateRequest{
		Spec: protoSpec,
	})
	if err != nil {
		log.Fatalf("could not create: %v", err)
	}
	fmt.Printf("Tunnel %q created\n", name)
}

func load(client pb.TunnelServiceClient, args []string) {
	if len(args) == 0 {
		log.Fatal("config file path required")
	}
	configPath := args[0]

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	specs, err := cfg.ToSpecs()
	if err != nil {
		log.Fatalf("failed to parse specs: %v", err)
	}

	for name, spec := range specs {
		_, err := client.Create(context.Background(), &pb.CreateRequest{
			Spec: spec.ToProto(),
		})
		if err != nil {
			fmt.Printf("Failed to create tunnel %q: %v\n", name, err)
			continue
		}

		_, err = client.Start(context.Background(), &pb.StartRequest{Name: name})
		if err != nil {
			fmt.Printf("Failed to start tunnel %q: %v\n", name, err)
			continue
		}
		fmt.Printf("Tunnel %q loaded and started\n", name)
	}
}

func deleteTunnel(client pb.TunnelServiceClient, args []string) {
	if len(args) == 0 {
		log.Fatal("tunnel name required")
	}
	_, err := client.Delete(context.Background(), &pb.DeleteRequest{Name: args[0]})
	if err != nil {
		log.Fatalf("could not delete: %v", err)
	}
	fmt.Printf("Tunnel %q deleted\n", args[0])
}
