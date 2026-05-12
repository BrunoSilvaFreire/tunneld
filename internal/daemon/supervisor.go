package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/BrunoSilvaFreire/tunneld/internal/health"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
)

type Supervisor struct {
	planner   *dependency.Planner
	processes map[string]*Process
	config    *config.Config
	keyDir    string
	mu        sync.RWMutex
	ctx       context.Context
}

func NewSupervisor(planner *dependency.Planner, cfg *config.Config, keyDir string) *Supervisor {
	return &Supervisor{
		planner:   planner,
		processes: make(map[string]*Process),
		config:    cfg,
		keyDir:    keyDir,
	}
}

func (s *Supervisor) GetKeyDir() string {
	return s.keyDir
}

func (s *Supervisor) SaveKey(name string, content []byte) error {
	if err := os.MkdirAll(s.keyDir, 0700); err != nil {
		return err
	}
	keyPath := filepath.Join(s.keyDir, name)
	return os.WriteFile(keyPath, content, 0600)
}

func (s *Supervisor) Run(ctx context.Context) error {
	s.ctx = ctx
	order, err := s.planner.Plan()
	if err != nil {
		return err
	}

	seen := map[string]string{}
	for _, spec := range order {
		for _, key := range tunnel.LocalPortKeys(spec.ToProto()) {
			if owner, ok := seen[key]; ok {
				return fmt.Errorf("port conflict in config: %q and %q both claim %s", spec.Name(), owner, key)
			}
			seen[key] = spec.Name()
		}
	}

	for _, spec := range order {
		p := NewProcess(spec, s.keyDir)
		s.mu.Lock()
		s.processes[spec.Name()] = p
		s.mu.Unlock()

		log.Printf("[%s] Initializing...", spec.Name())
		if err := s.startAndNotify(ctx, p); err != nil {
			log.Printf("[%s] Initial start failed: %v", spec.Name(), err)
		}
	}

	<-ctx.Done()
	return s.StopAll()
}

type TunnelInfo struct {
	Name         string
	Status       tunnel.Status
	Error        string
	Spec         tunnel.Spec
	PortMappings []tunnel.PortMapping
}

func (s *Supervisor) GetProcess(name string) (*Process, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.processes[name]
	if !ok {
		return nil, fmt.Errorf("tunnel %q not found", name)
	}
	return p, nil
}

func (s *Supervisor) ListTunnels() []TunnelInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var infos []TunnelInfo
	for name, p := range s.processes {
		errStr := ""
		if p.LastError() != nil {
			errStr = p.LastError().Error()
		}
		infos = append(infos, TunnelInfo{
			Name:         name,
			Status:       p.Status(),
			Error:        errStr,
			Spec:         p.spec,
			PortMappings: p.PortMappings(),
		})
	}
	return infos
}

func (s *Supervisor) checkPortConflicts(spec tunnel.Spec) error {
	newKeys := tunnel.LocalPortKeys(spec.ToProto())
	for _, existing := range s.processes {
		for _, key := range tunnel.LocalPortKeys(existing.spec.ToProto()) {
			for _, nk := range newKeys {
				if key == nk {
					return fmt.Errorf("port conflict: %q and %q both claim %s",
						spec.Name(), existing.spec.Name(), key)
				}
			}
		}
	}
	return nil
}

func (s *Supervisor) AddTunnel(ctx context.Context, spec tunnel.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.processes[spec.Name()]; ok {
		return fmt.Errorf("tunnel %q already exists", spec.Name())
	}

	if err := s.checkPortConflicts(spec); err != nil {
		return err
	}

	s.planner.AddSpec(spec)
	if _, err := s.planner.Plan(); err != nil {
		s.planner.RemoveSpec(spec.Name())
		return fmt.Errorf("invalid dependencies: %v", err)
	}

	s.processes[spec.Name()] = NewProcess(spec, s.keyDir)
	return nil
}

func (s *Supervisor) RemoveTunnel(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.processes[name]
	if !ok {
		return fmt.Errorf("tunnel %q not found", name)
	}

	p.Stop()
	delete(s.processes, name)
	s.planner.RemoveSpec(name)
	return nil
}

func (s *Supervisor) EnableTunnel(ctx context.Context, name string) error {
	s.mu.Lock()
	p, ok := s.processes[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("tunnel %q not found", name)
	}

	if err := s.config.SetEnabled(name, true); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to update config: %v", err)
	}
	s.mu.Unlock()

	return s.startAndNotify(s.ctx, p)
}

func (s *Supervisor) DisableTunnel(name string) error {
	s.mu.Lock()
	p, ok := s.processes[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("tunnel %q not found", name)
	}

	if err := s.config.SetEnabled(name, false); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to update config: %v", err)
	}
	s.mu.Unlock()

	return p.Stop()
}

func (s *Supervisor) StartTunnel(ctx context.Context, name string) error {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tunnel %q not found", name)
	}

	status := p.Status()
	if status == tunnel.StatusRunning || status == tunnel.StatusStarting {
		// If it's already running or starting, just return success (idempotent)
		// but check if the underlying process is actually alive.
		if p.IsProcessAlive() {
			return nil
		}
		// If process is dead but status is Running/Starting, we allow a restart.
	}

	return s.startAndNotify(s.ctx, p)
}

func (s *Supervisor) StopTunnel(name string) error {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tunnel %q not found", name)
	}

	return p.Stop()
}

func (s *Supervisor) WaitHealthy(ctx context.Context, name string, timeout time.Duration) (tunnel.Status, error) {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tunnel %q not found", name)
	}

	if p.Status() == tunnel.StatusRunning {
		return tunnel.StatusRunning, nil
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return p.Status(), ctx.Err()
		case <-ticker.C:
			status := p.Status()
			if status == tunnel.StatusRunning {
				return tunnel.StatusRunning, nil
			}
			if status == tunnel.StatusFailed {
				policy := p.spec.RestartPolicy()
				if policy == nil || policy.Policy == "never" {
					return tunnel.StatusFailed, fmt.Errorf("tunnel failed")
				}
				// Tunnel has a restart policy; continue polling through transient failures.
			}
			if time.Now().After(deadline) {
				return status, fmt.Errorf("timeout waiting for health")
			}
		}
	}
}

func (s *Supervisor) startAndNotify(ctx context.Context, p *Process) error {
	spec := p.spec
	log.Printf("[%s] Starting...", spec.Name())

	if err := p.Start(ctx); err != nil {
		return err
	}

	// Wait for health
	hSpec := p.EffectiveHealthCheck()
	h := health.NewChecker(hSpec)
	start := time.Now()
	timeout := hSpec.GetStartupTimeout().AsDuration()
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	go func() {
		interval := hSpec.GetInterval().AsDuration()
		if interval <= 0 {
			interval = 2 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-p.Wait():
				log.Printf("[%s] Process exited unexpectedly with error: %v", spec.Name(), err)
				s.handleFailure(ctx, spec.Name())
				return
			case <-ticker.C:
				checkTimeout := hSpec.GetTimeout().AsDuration()
				if checkTimeout == 0 {
					checkTimeout = 2 * time.Second
				}
				checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
				err := h.Check(checkCtx)
				cancel()

				if err == nil {
					if p.Status() != tunnel.StatusRunning {
						log.Printf("[%s] Healthy and running", spec.Name())
						p.setStatus(tunnel.StatusRunning)
						p.setError(nil)
					}
				} else {
					if time.Since(start) > timeout {
						log.Printf("[%s] Health check failed after timeout: %v", spec.Name(), err)
						p.setError(fmt.Errorf("health check timeout: %v", err))
						p.setStatus(tunnel.StatusFailed)
						s.handleFailure(ctx, spec.Name())
						return
					}
				}
			}
		}
	}()

	return nil
}

func (s *Supervisor) handleFailure(ctx context.Context, name string) {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()
	if !ok {
		return
	}

	log.Printf("[%s] Handling failure...", name)

	if p.ExpectedState() == "stopped" {
		log.Printf("[%s] Expected state is stopped, skipping restart", name)
		return
	}

	// 1. Find and stop dependents
	s.mu.RLock()
	deps, err := s.planner.DependentsOf(name)
	s.mu.RUnlock()
	if err == nil {
		for _, depName := range deps {
			s.mu.RLock()
			depP, ok := s.processes[depName]
			s.mu.RUnlock()
			if ok && depP.Status() != tunnel.StatusStopped {
				log.Printf("[%s] Stopping dependent %s due to upstream failure", name, depName)
				depP.Stop()
			}
		}
	}

	// 2. Restart policy
	policy := p.spec.RestartPolicy()
	if policy.Policy == "never" {
		return
	}

	// Simple restart logic for v1
	time.Sleep(policy.Delay.AsDuration())
	log.Printf("[%s] Attempting restart...", name)
	if err := s.startAndNotify(ctx, p); err != nil {
		log.Printf("[%s] Restart failed: %v", name, err)
	}
}

func (s *Supervisor) StopAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Println("Stopping all tunnels...")
	for _, p := range s.processes {
		p.Stop()
	}
	return nil
}
