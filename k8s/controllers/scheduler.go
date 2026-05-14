package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// pickNode returns the name and internal IP of a Ready node matching selector.
// Selection is intentionally simple: lexicographically first matching node, so
// scheduling is stable across reconciles. We do NOT attempt load balancing in
// v1alpha1 — operators usually pin tunnels to specific nodes via labels anyway.
func pickNode(ctx context.Context, c client.Client, selector map[string]string) (name, ip string, err error) {
	var nodes corev1.NodeList
	if err := c.List(ctx, &nodes); err != nil {
		return "", "", fmt.Errorf("list nodes: %w", err)
	}

	sel := labels.SelectorFromSet(selector)
	var chosen *corev1.Node
	for i := range nodes.Items {
		n := &nodes.Items[i]
		if !sel.Matches(labels.Set(n.Labels)) {
			continue
		}
		if !isNodeReady(n) {
			continue
		}
		if chosen == nil || n.Name < chosen.Name {
			chosen = n
		}
	}
	if chosen == nil {
		return "", "", fmt.Errorf("no Ready node matches selector %v", selector)
	}
	return chosen.Name, nodeInternalIP(chosen), nil
}

// nodeInternalIP returns the first InternalIP address from a node's status,
// falling back to ExternalIP if no internal address is present.
func nodeInternalIP(n *corev1.Node) string {
	var ext string
	for _, a := range n.Status.Addresses {
		switch a.Type {
		case corev1.NodeInternalIP:
			return a.Address
		case corev1.NodeExternalIP:
			if ext == "" {
				ext = a.Address
			}
		}
	}
	return ext
}

func isNodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
