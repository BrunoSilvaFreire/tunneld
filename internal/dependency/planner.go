package dependency

import (
	"fmt"
	"github.com/BrunoSilvaFreire/gographs/pkg"
	"github.com/BrunoSilvaFreire/gographs/pkg/algorithms"
	"github.com/BrunoSilvaFreire/tunneld/internal/tunnel"
)

type Planner struct {
	specs map[string]tunnel.Spec
	graph *pkg.AdjacencyList[tunnel.Spec, struct{}]
	indices map[string]pkg.GraphIndex
}

func NewPlanner(specs map[string]tunnel.Spec) *Planner {
	return &Planner{
		specs:   specs,
		graph:   pkg.NewAdjacencyList[tunnel.Spec, struct{}](),
		indices: make(map[string]pkg.GraphIndex),
	}
}

func (p *Planner) AddSpec(spec tunnel.Spec) {
	if p.specs == nil {
		p.specs = make(map[string]tunnel.Spec)
	}
	p.specs[spec.Name()] = spec
}

func (p *Planner) RemoveSpec(name string) {
	delete(p.specs, name)
}

func (p *Planner) Plan() ([]tunnel.Spec, error) {
	p.graph = pkg.NewAdjacencyList[tunnel.Spec, struct{}]()
	p.indices = make(map[string]pkg.GraphIndex)

	// Add all vertices
	for name, spec := range p.specs {
		p.indices[name] = p.graph.AddVertex(spec)
	}

	// Add edges (dependency -> dependent)
	// Wait, the user wants: "ssh-bastion -> kube-api -> service-port-forward"
	// This means kube-api depends on ssh-bastion.
	// In topological sort, we want dependencies to come BEFORE dependents.
	// So edges should be dependency -> dependent.
	for name, spec := range p.specs {
		toIdx := p.indices[name]
		for _, depName := range spec.Dependencies() {
			fromIdx, ok := p.indices[depName]
			if !ok {
				return nil, fmt.Errorf("tunnel %q depends on missing tunnel %q", name, depName)
			}
			// Edge: dependency -> dependent
			if err := p.graph.Connect(fromIdx, toIdx, struct{}{}); err != nil {
				return nil, err
			}
		}
	}

	orderIndices, err := algorithms.TopologicalSort(p.graph)
	if err != nil {
		if err == pkg.ErrCycleDetected {
			return nil, fmt.Errorf("dependency cycle detected")
		}
		return nil, err
	}

	result := make([]tunnel.Spec, 0, len(orderIndices))
	for _, idx := range orderIndices {
		v, _ := p.graph.Vertex(idx)
		result = append(result, *v)
	}

	return result, nil
}

// DependentsOf returns all tunnels that depend (directly or indirectly) on the given tunnel name.
func (p *Planner) DependentsOf(name string) ([]string, error) {
	startIdx, ok := p.indices[name]
	if !ok {
		return nil, fmt.Errorf("tunnel %q not found", name)
	}

	// We can use BFS/DFS starting from startIdx because edges are dependency -> dependent.
	var dependents []string
	// Flood is BFS with multiple starts.
	for idx := range algorithms.Flood(p.graph, startIdx) {
		if idx == startIdx {
			continue
		}
		v, _ := p.graph.Vertex(idx)
		dependents = append(dependents, (*v).Name())
	}

	return dependents, nil
}
