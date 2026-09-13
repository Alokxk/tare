// Package cluster owns every connection Tare makes to a Kubernetes API server.
package cluster

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Tare lists several resource types across every namespace in one pass.
// client-go's 5 QPS default is tuned for a controller's steady trickle and
// throttles that pass badly, so it is raised deliberately here.
const (
	clientQPS     = 50
	clientBurst   = 100
	clientTimeout = 30 * time.Second
)

type Options struct {
	KubeconfigPath string
	Context        string
	UserAgent      string
}

// Client is the only route from Tare to an API server. clientset is unexported
// on purpose: no package outside this one can reach a write method on it, so
// adding a method to this file is the single way a write could ever enter Tare.
// Read methods are Get and List only, never Watch.
type Client struct {
	clientset kubernetes.Interface
}

func Connect(opts Options) (*Client, error) {
	// The default rules implement the whole precedence chain, including
	// splitting KUBECONFIG on the path separator and merging those files.
	// Setting ExplicitPath short-circuits it, so only do that when asked.
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.KubeconfigPath != "" {
		rules.ExplicitPath = opts.KubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		overrides.CurrentContext = opts.Context
	}

	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	restCfg.QPS = clientQPS
	restCfg.Burst = clientBurst
	restCfg.Timeout = clientTimeout
	if opts.UserAgent != "" {
		restCfg.UserAgent = opts.UserAgent
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("building clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// ServerVersion is a GET against /version. It doubles as Tare's connectivity
// check: it needs credentials to succeed but no resource permissions, so a
// failure here is a connection problem and never an RBAC one.
func (c *Client) ServerVersion() (*version.Info, error) {
	return c.clientset.Discovery().ServerVersion()
}

// The API server returns every result in one response when Limit is unset,
// which is a large allocation on a big cluster. Paging bounds each response;
// the Continue loop in listAll keeps the overall result complete.
const listPageSize = 500

// listAll drains a paginated List. page fetches one page and returns its items
// plus the server's continue token, which is empty on the final page. Skipping
// this loop is the classic client-go bug: a Limit with no Continue silently
// truncates the result and reports it as complete.
func listAll[T any](page func(metav1.ListOptions) ([]T, string, error)) ([]T, error) {
	var out []T
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		items, cont, err := page(opts)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if cont == "" {
			return out, nil
		}
		opts.Continue = cont
	}
}

// An empty namespace lists across the whole cluster: metav1.NamespaceAll is
// itself "", so the flag default and the API convention coincide.
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	return listAll(func(opts metav1.ListOptions) ([]appsv1.Deployment, string, error) {
		l, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, opts)
		if err != nil {
			return nil, "", fmt.Errorf("listing deployments: %w", err)
		}
		return l.Items, l.Continue, nil
	})
}

func (c *Client) ListStatefulSets(ctx context.Context, namespace string) ([]appsv1.StatefulSet, error) {
	return listAll(func(opts metav1.ListOptions) ([]appsv1.StatefulSet, string, error) {
		l, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, opts)
		if err != nil {
			return nil, "", fmt.Errorf("listing statefulsets: %w", err)
		}
		return l.Items, l.Continue, nil
	})
}

func (c *Client) ListDaemonSets(ctx context.Context, namespace string) ([]appsv1.DaemonSet, error) {
	return listAll(func(opts metav1.ListOptions) ([]appsv1.DaemonSet, string, error) {
		l, err := c.clientset.AppsV1().DaemonSets(namespace).List(ctx, opts)
		if err != nil {
			return nil, "", fmt.Errorf("listing daemonsets: %w", err)
		}
		return l.Items, l.Continue, nil
	})
}
