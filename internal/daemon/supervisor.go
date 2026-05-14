package daemon

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BrunoSilvaFreire/tunneld/internal/config"
	"github.com/BrunoSilvaFreire/tunneld/internal/constants"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/BrunoSilvaFreire/tunneld/internal/health"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
)

type Supervisor struct {
	planner    *dependency.Planner
	processes  map[string]*Process
	config     *config.Config
	keyDir     string
	tunnelsDir string
	mu         sync.RWMutex
	ctx        context.Context
}

func NewSupervisor(planner *dependency.Planner, cfg *config.Config, keyDir, tunnelsDir string) *Supervisor {
	return &Supervisor{
		planner:    planner,
		processes:  make(map[string]*Process),
		config:     cfg,
		keyDir:     keyDir,
		tunnelsDir: tunnelsDir,
	}
}

func (s *Supervisor) GetKeyDir() string {
	return s.keyDir
}

func (s *Supervisor) SaveKey(name string, content []byte) error {
	if err := os.MkdirAll(s.keyDir, constants.PermDirPrivate); err != nil {
		return err
	}
	keyPath := filepath.Join(s.keyDir, name)
	return os.WriteFile(keyPath, content, constants.PermFilePrivate)
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

func (s *Supervisor) AddTunnel(ctx context.Context, spec tunnel.Spec, persistent bool) error {
	s.mu.Lock()
	if _, ok := s.processes[spec.Name()]; ok {
		s.mu.Unlock()
		return fmt.Errorf("tunnel %q already exists", spec.Name())
	}

	if err := s.checkPortConflicts(spec); err != nil {
		s.mu.Unlock()
		return err
	}

	if persistent && s.tunnelsDir != "" {
		if err := os.MkdirAll(s.tunnelsDir, constants.PermDirPublic); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to create tunnels directory: %v", err)
		}
		path := filepath.Join(s.tunnelsDir, fmt.Sprintf("%s.yaml", spec.Name()))
		data, err := config.MarshalTunnel(spec.ToProto())
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to marshal tunnel: %v", err)
		}
		if err := os.WriteFile(path, data, constants.PermFilePublic); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to save tunnel config: %v", err)
		}
	}

	s.planner.AddSpec(spec)
	if _, err := s.planner.Plan(); err != nil {
		s.planner.RemoveSpec(spec.Name())
		s.mu.Unlock()
		return fmt.Errorf("invalid dependencies: %v", err)
	}

	p := NewProcess(spec, s.keyDir)
	s.processes[spec.Name()] = p
	runCtx := s.ctx
	s.mu.Unlock()

	if runCtx != nil {
		return s.startAndNotify(runCtx, p)
	}
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

	if timeout == 0 {
		timeout = constants.DefaultWaitTimeout
	}
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
		timeout = constants.DefaultStartupTimeout
	}

	go func() {
		interval := hSpec.GetInterval().AsDuration()
		if interval <= 0 {
			interval = constants.DefaultHealthInterval
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
					checkTimeout = constants.DefaultHealthTimeout
				}
				checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
				err := h.Check(checkCtx)
				cancel()

				if err == nil {
					if p.Status() != tunnel.StatusRunning {
						log.Printf("[%s] Healthy and running", spec.Name())
						p.setStatus(tunnel.StatusRunning)
						p.setError(nil)
						p.resetRestartAttempts()
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

	if p.ExpectedState() == DesiredStopped || p.ExpectedState() == DesiredDisabled {
		log.Printf("[%s] Expected state is %s, skipping restart", name, p.ExpectedState())
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
	if policy == nil || policy.Policy == "never" {
		return
	}

	attempts := p.RestartAttempts()
	if policy.MaxAttempts > 0 && int32(attempts) >= policy.MaxAttempts {
		log.Printf("[%s] Max restart attempts (%d) reached", name, policy.MaxAttempts)
		p.setStatus(tunnel.StatusFailed)
		p.setError(fmt.Errorf("max restart attempts reached"))
		return
	}

	delay := policy.Delay.AsDuration()
	if policy.Backoff != nil && attempts > 0 {
		multiplier := policy.Backoff.Multiplier
		if multiplier <= 0 {
			multiplier = 2
		}
		backoffDelay := time.Duration(float64(delay) * math.Pow(float64(multiplier), float64(attempts)))
		maxDelay := policy.Backoff.MaxDelay.AsDuration()
		if maxDelay > 0 && backoffDelay > maxDelay {
			backoffDelay = maxDelay
		}
		delay = backoffDelay
	}

	log.Printf("[%s] Attempting restart %d in %v...", name, attempts+1, delay)
	p.incrementRestartAttempts()
	time.Sleep(delay)
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
