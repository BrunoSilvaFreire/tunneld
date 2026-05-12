package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
	"google.golang.org/protobuf/proto"
)

type Process struct {
	spec          tunnel.Spec
	cmd           *exec.Cmd
	status        tunnel.Status
	lastError     error
	mu            sync.RWMutex
	cancel        context.CancelFunc
	waitCh        chan error
	logPath       string
	expectedState string
	keyDir        string
	portMappings  []tunnel.PortMapping
}

func NewProcess(spec tunnel.Spec, keyDir string) *Process {
	logDir := filepath.Join(os.TempDir(), "tunneld-logs")
	return &Process{
		spec:          spec,
		status:        tunnel.StatusStopped,
		logPath:       filepath.Join(logDir, fmt.Sprintf("%s.log", spec.Name())),
		expectedState: "stopped",
		keyDir:        keyDir,
	}
}

func (p *Process) Status() tunnel.Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Process) ExpectedState() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.expectedState
}

func (p *Process) LastError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

func (p *Process) setStatus(s tunnel.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

func (p *Process) setExpectedState(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expectedState = s
}

func (p *Process) setError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err
}

func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.status != tunnel.StatusStopped && p.status != tunnel.StatusFailed {
		p.mu.Unlock()
		return fmt.Errorf("process already running")
	}
	p.expectedState = "running"
	p.mu.Unlock()

	// Ensure log directory exists
	logDir := filepath.Dir(p.logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open log file in append mode
	logFile, err := os.OpenFile(p.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	resolvedProto, mappings, err := tunnel.ResolvePortsInProto(p.spec.ToProto())
	if err != nil {
		logFile.Close()
		cancel()
		return fmt.Errorf("resolve ports: %w", err)
	}
	p.mu.Lock()
	p.portMappings = mappings
	p.mu.Unlock()

	resolvedSpec, err := tunnel.FromProto(resolvedProto)
	if err != nil {
		logFile.Close()
		cancel()
		return fmt.Errorf("build resolved spec: %w", err)
	}
	cmd, err := resolvedSpec.BuildCommand(runCtx, p.keyDir)
	if err != nil {
		logFile.Close()
		cancel()
		return err
	}
	p.cmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logFile.Close()
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logFile.Close()
		cancel()
		return err
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		cancel()
		return err
	}

	p.setStatus(tunnel.StatusStarting)
	p.setError(nil)
	p.waitCh = make(chan error, 1)

	logger := log.New(logFile, "", log.LstdFlags)

	// Stream logs
	go p.streamLogs(stdout, "stdout", logger)
	go p.streamLogs(stderr, "stderr", logger)

	// Wait for exit
	go func() {
		defer logFile.Close()
		err := cmd.Wait()
		if err != nil {
			p.setError(err)
			p.setStatus(tunnel.StatusFailed)
		} else {
			p.setStatus(tunnel.StatusStopped)
		}
		p.waitCh <- err
	}()

	return nil
}

func (p *Process) Stop() error {
	p.setExpectedState("stopped")
	if p.cancel != nil {
		p.cancel()
	}

	if p.cmd != nil && p.cmd.Process != nil {
		// Wait for graceful exit or timeout
		select {
		case <-p.waitCh:
			return nil
		case <-time.After(p.spec.ShutdownTimeout()):
			if p.spec.ShutdownTimeout() == 0 {
				return nil
			}
			log.Printf("[%s] Shutdown timeout reached, killing process", p.spec.Name())
			return p.cmd.Process.Kill()
		}
	}
	return nil
}

func (p *Process) Wait() <-chan error {
	return p.waitCh
}

func (p *Process) LogPath() string {
	return p.logPath
}

func (p *Process) IsProcessAlive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cmd != nil && p.cmd.Process != nil && p.cmd.ProcessState == nil
}

func (p *Process) PortMappings() []tunnel.PortMapping {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.portMappings
}

// EffectiveHealthCheck returns the health check spec with any port=0 address
// updated to the actual OS-assigned port after dynamic allocation.
func (p *Process) EffectiveHealthCheck() *pb.HealthCheckSpec {
	h := p.spec.HealthCheck()
	if h == nil || h.Address == "" {
		return h
	}
	host, portStr, err := net.SplitHostPort(h.Address)
	if err != nil || portStr != "0" {
		return h
	}
	p.mu.RLock()
	mappings := p.portMappings
	p.mu.RUnlock()
	for _, m := range mappings {
		if m.LocalAddress == host && m.ConfiguredPort == 0 {
			hCopy := proto.Clone(h).(*pb.HealthCheckSpec)
			hCopy.Address = net.JoinHostPort(host, strconv.Itoa(int(m.ActualPort)))
			return hCopy
		}
	}
	return h
}

func (p *Process) streamLogs(r io.Reader, label string, logger *log.Logger) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			msg := fmt.Sprintf("[%s][%s] %s", p.spec.Name(), label, string(buf[:n]))
			fmt.Print(msg)
			logger.Print(msg)
		}
		if err != nil {
			break
		}
	}
}
