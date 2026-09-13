package cluster

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListAll(t *testing.T) {
	t.Run("follows continue tokens", func(t *testing.T) {
		pages := [][]int{{1, 2}, {3, 4}, {5}}
		tokens := []string{"tok-a", "tok-b", ""}

		var seen []metav1.ListOptions
		call := 0
		got, err := listAll(func(opts metav1.ListOptions) ([]int, string, error) {
			seen = append(seen, opts)
			items, tok := pages[call], tokens[call]
			call++
			return items, tok, nil
		})
		if err != nil {
			t.Fatalf("listAll: %v", err)
		}

		want := []int{1, 2, 3, 4, 5}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
		if len(seen) != 3 {
			t.Fatalf("made %d calls, want 3", len(seen))
		}
		for i, opts := range seen {
			if opts.Limit != listPageSize {
				t.Errorf("call %d Limit = %d, want %d", i, opts.Limit, listPageSize)
			}
		}
		if seen[0].Continue != "" {
			t.Errorf("call 0 Continue = %q, want empty", seen[0].Continue)
		}
		if seen[1].Continue != "tok-a" || seen[2].Continue != "tok-b" {
			t.Errorf("continue tokens not fed back: %q, %q", seen[1].Continue, seen[2].Continue)
		}
	})

	t.Run("single page", func(t *testing.T) {
		calls := 0
		got, err := listAll(func(metav1.ListOptions) ([]int, string, error) {
			calls++
			return []int{7}, "", nil
		})
		if err != nil || len(got) != 1 || calls != 1 {
			t.Fatalf("got %v, err %v, calls %d", got, err, calls)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		sentinel := errors.New("boom")
		_, err := listAll(func(metav1.ListOptions) ([]int, string, error) {
			return nil, "", sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want %v", err, sentinel)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		got, err := listAll(func(metav1.ListOptions) ([]int, string, error) {
			return nil, "", nil
		})
		if err != nil || got != nil {
			t.Fatalf("got %v, err %v", got, err)
		}
	})
}

func TestListWorkloads(t *testing.T) {
	client := &Client{clientset: fake.NewClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "staging"}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "prod"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "logs", Namespace: "kube-system"}},
	)}
	ctx := context.Background()

	deployments, err := client.ListDeployments(ctx, "")
	if err != nil || len(deployments) != 2 {
		t.Fatalf("deployments = %d, err %v", len(deployments), err)
	}

	scoped, err := client.ListDeployments(ctx, "prod")
	if err != nil || len(scoped) != 1 || scoped[0].Name != "web" {
		t.Fatalf("scoped = %+v, err %v", scoped, err)
	}

	statefulSets, err := client.ListStatefulSets(ctx, "")
	if err != nil || len(statefulSets) != 1 {
		t.Fatalf("statefulsets = %d, err %v", len(statefulSets), err)
	}

	daemonSets, err := client.ListDaemonSets(ctx, "")
	if err != nil || len(daemonSets) != 1 {
		t.Fatalf("daemonsets = %d, err %v", len(daemonSets), err)
	}
}
