package daemon

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/dependency"
	"github.com/BrunoSilvaFreire/tunneld/internal/health"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
)

type Supervisor struct {
	planner   *dependency.Planner
	processes map[string]*Process
	mu        sync.RWMutex
	ctx       context.Context
}

func NewSupervisor(planner *dependency.Planner) *Supervisor {
	return &Supervisor{
		planner:   planner,
		processes: make(map[string]*Process),
	}
}

func (s *Supervisor) Run(ctx context.Context) error {
	s.ctx = ctx
	order, err := s.planner.Plan()
	if err != nil {
		return err
	}

	for _, spec := range order {
		p := NewProcess(spec)
		s.mu.Lock()
		s.processes[spec.Name()] = p
		s.mu.Unlock()

		log.Printf("[%s] Initializing...", spec.Name())
		// Only start if it was in initial config and we want it to start on boot
		// In v1, we start everything in the initial planner order if Run is called.
		if err := s.startAndNotify(ctx, p); err != nil {
			log.Printf("[%s] Initial start failed: %v", spec.Name(), err)
		}
	}

	<-ctx.Done()
	return s.StopAll()
}

type TunnelInfo struct {
	Name   string
	Status tunnel.Status
	Spec   tunnel.Spec
}

func (s *Supervisor) ListTunnels() []TunnelInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var infos []TunnelInfo
	for name, p := range s.processes {
		infos = append(infos, TunnelInfo{
			Name:   name,
			Status: p.Status(),
			Spec:   p.spec,
		})
	}
	return infos
}

func (s *Supervisor) AddTunnel(ctx context.Context, spec tunnel.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.processes[spec.Name()]; ok {
		return fmt.Errorf("tunnel %q already exists", spec.Name())
	}

	s.planner.AddSpec(spec)
	if _, err := s.planner.Plan(); err != nil {
		s.planner.RemoveSpec(spec.Name())
		return fmt.Errorf("invalid dependencies: %v", err)
	}

	s.processes[spec.Name()] = NewProcess(spec)
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

func (s *Supervisor) StartTunnel(ctx context.Context, name string) error {
	s.mu.RLock()
	p, ok := s.processes[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tunnel %q not found", name)
	}

	return s.startAndNotify(ctx, p)
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
				return tunnel.StatusFailed, fmt.Errorf("tunnel failed")
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
	h := health.NewChecker(spec.HealthCheck())
	start := time.Now()
	timeout := spec.HealthCheck().GetStartupTimeout().AsDuration()
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	go func() {
		interval := spec.HealthCheck().GetInterval().AsDuration()
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
				checkTimeout := spec.HealthCheck().GetTimeout().AsDuration()
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
					}
				} else {
					if time.Since(start) > timeout {
						log.Printf("[%s] Health check failed after timeout: %v", spec.Name(), err)
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
