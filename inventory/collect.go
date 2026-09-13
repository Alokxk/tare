// Package inventory turns the cluster's workload objects into one flat,
// normalised list that the rest of Tare reports on.
package inventory

import (
	"cmp"
	"context"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
)

type Kind string

const (
	KindDeployment  Kind = "Deployment"
	KindStatefulSet Kind = "StatefulSet"
	KindDaemonSet   Kind = "DaemonSet"
)

type Workload struct {
	Kind      Kind
	Namespace string
	Name      string
	Resources Resources
}

// Lister is declared here rather than in package cluster so this package can
// be tested against a stub. *cluster.Client satisfies it implicitly.
type Lister interface {
	ListDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error)
	ListStatefulSets(ctx context.Context, namespace string) ([]appsv1.StatefulSet, error)
	ListDaemonSets(ctx context.Context, namespace string) ([]appsv1.DaemonSet, error)
}

// Collect lists every workload in namespace, or cluster-wide when it is empty.
func Collect(ctx context.Context, l Lister, namespace string) ([]Workload, error) {
	var out []Workload

	deployments, err := l.ListDeployments(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, d := range deployments {
		out = append(out, Workload{
			Kind: KindDeployment, Namespace: d.Namespace, Name: d.Name,
			Resources: podResources(d.Spec.Template.Spec),
		})
	}

	statefulSets, err := l.ListStatefulSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, s := range statefulSets {
		out = append(out, Workload{
			Kind: KindStatefulSet, Namespace: s.Namespace, Name: s.Name,
			Resources: podResources(s.Spec.Template.Spec),
		})
	}

	daemonSets, err := l.ListDaemonSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, d := range daemonSets {
		out = append(out, Workload{
			Kind: KindDaemonSet, Namespace: d.Namespace, Name: d.Name,
			Resources: podResources(d.Spec.Template.Spec),
		})
	}

	// The API server promises no ordering. A report that is screenshotted and
	// re-run needs the same order every time.
	slices.SortFunc(out, func(a, b Workload) int {
		return cmp.Or(
			cmp.Compare(a.Namespace, b.Namespace),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return out, nil
}
