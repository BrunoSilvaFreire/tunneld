package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
)

type Process struct {
	spec   tunnel.Spec
	cmd    *exec.Cmd
	status tunnel.Status
	mu     sync.RWMutex
	cancel context.CancelFunc
	waitCh chan error
}

func NewProcess(spec tunnel.Spec) *Process {
	return &Process{
		spec:   spec,
		status: tunnel.StatusStopped,
	}
}

func (p *Process) Status() tunnel.Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Process) setStatus(s tunnel.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.status != tunnel.StatusStopped && p.status != tunnel.StatusFailed {
		p.mu.Unlock()
		return fmt.Errorf("process already running")
	}
	p.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	cmd, err := p.spec.BuildCommand(runCtx)
	if err != nil {
		cancel()
		return err
	}
	p.cmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}

	p.setStatus(tunnel.StatusStarting)
	p.waitCh = make(chan error, 1)

	// Stream logs
	go p.streamLogs(stdout, "stdout")
	go p.streamLogs(stderr, "stderr")

	// Wait for exit
	go func() {
		err := cmd.Wait()
		p.setStatus(tunnel.StatusStopped)
		p.waitCh <- err
	}()

	return nil
}

func (p *Process) Stop() error {
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

func (p *Process) streamLogs(r io.Reader, label string) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			fmt.Printf("[%s][%s] %s", p.spec.Name(), label, string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
}
