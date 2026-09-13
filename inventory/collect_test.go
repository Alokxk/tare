package inventory

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stubLister struct {
	deployments  []appsv1.Deployment
	statefulSets []appsv1.StatefulSet
	daemonSets   []appsv1.DaemonSet
	err          error
}

func (s stubLister) ListDeployments(context.Context, string) ([]appsv1.Deployment, error) {
	return s.deployments, s.err
}

func (s stubLister) ListStatefulSets(context.Context, string) ([]appsv1.StatefulSet, error) {
	return s.statefulSets, s.err
}

func (s stubLister) ListDaemonSets(context.Context, string) ([]appsv1.DaemonSet, error) {
	return s.daemonSets, s.err
}

func meta(ns, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: ns, Name: name}
}

func TestCollect(t *testing.T) {
	tests := []struct {
		name   string
		lister stubLister
		want   []Workload
	}{
		{
			name:   "empty cluster",
			lister: stubLister{},
			want:   nil,
		},
		{
			name: "sorts by namespace then kind then name",
			lister: stubLister{
				deployments: []appsv1.Deployment{
					{ObjectMeta: meta("prod", "web")},
					{ObjectMeta: meta("alpha", "api")},
				},
				statefulSets: []appsv1.StatefulSet{
					{ObjectMeta: meta("prod", "db")},
				},
				daemonSets: []appsv1.DaemonSet{
					{ObjectMeta: meta("alpha", "logs")},
				},
			},
			want: []Workload{
				{KindDaemonSet, "alpha", "logs"},
				{KindDeployment, "alpha", "api"},
				{KindDeployment, "prod", "web"},
				{KindStatefulSet, "prod", "db"},
			},
		},
		{
			name: "same kind same namespace sorts by name",
			lister: stubLister{
				deployments: []appsv1.Deployment{
					{ObjectMeta: meta("prod", "zeta")},
					{ObjectMeta: meta("prod", "alpha")},
				},
			},
			want: []Workload{
				{KindDeployment, "prod", "alpha"},
				{KindDeployment, "prod", "zeta"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Collect(context.Background(), tt.lister, "")
			if err != nil {
				t.Fatalf("Collect: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d workloads %+v, want %d", len(got), got, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCollectPropagatesError(t *testing.T) {
	sentinel := errors.New("forbidden")
	_, err := Collect(context.Background(), stubLister{err: sentinel}, "")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}
