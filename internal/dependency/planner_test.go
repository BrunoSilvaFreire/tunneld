package dependency

import (
	"context"
	"os/exec"
	"testing"
	"time"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
	pb "github.com/BrunoSilvaFreire/tunneld/pkg/api/v1"
)

type mockSpec struct {
	name string
	deps []string
}

func (m mockSpec) Name() string                     { return m.name }
func (m mockSpec) Type() string                     { return "mock" }
func (m mockSpec) Dependencies() []string           { return m.deps }
func (m mockSpec) BuildCommand(context.Context) (*exec.Cmd, error) { return nil, nil }
func (m mockSpec) HealthCheck() *pb.HealthCheckSpec { return &pb.HealthCheckSpec{} }
func (m mockSpec) RestartPolicy() *pb.RestartPolicySpec { return &pb.RestartPolicySpec{} }
func (m mockSpec) StartupTimeout() time.Duration    { return 0 }
func (m mockSpec) ShutdownTimeout() time.Duration   { return 0 }
func (m mockSpec) ToProto() *pb.TunnelSpec          { return &pb.TunnelSpec{Name: m.name, DependsOn: m.deps} }

func TestPlanner(t *testing.T) {
	specs := map[string]tunnel.Spec{
		"bastion": mockSpec{name: "bastion", deps: nil},
		"kube":    mockSpec{name: "kube", deps: []string{"bastion"}},
		"svc":     mockSpec{name: "svc", deps: []string{"kube"}},
	}

	planner := NewPlanner(specs)
	order, err := planner.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 tunnels, got %d", len(order))
	}

	if order[0].Name() != "bastion" || order[1].Name() != "kube" || order[2].Name() != "svc" {
		t.Errorf("incorrect order: %v", order)
	}

	deps, err := planner.DependentsOf("bastion")
	if err != nil {
		t.Fatalf("DependentsOf failed: %v", err)
	}
	// Dependents of bastion are kube and svc.
	if len(deps) != 2 {
		t.Errorf("expected 2 dependents, got %d: %v", len(deps), deps)
	}
}

func TestPlannerCycle(t *testing.T) {
	specs := map[string]tunnel.Spec{
		"a": mockSpec{name: "a", deps: []string{"b"}},
		"b": mockSpec{name: "b", deps: []string{"a"}},
	}

	planner := NewPlanner(specs)
	_, err := planner.Plan()
	if err == nil || err.Error() != "dependency cycle detected" {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestPlannerMissing(t *testing.T) {
	specs := map[string]tunnel.Spec{
		"a": mockSpec{name: "a", deps: []string{"missing"}},
	}

	planner := NewPlanner(specs)
	_, err := planner.Plan()
	if err == nil || err.Error() != `tunnel "a" depends on missing tunnel "missing"` {
		t.Fatalf("expected missing dependency error, got %v", err)
	}
}
