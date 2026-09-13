package inventory

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func loadSpec(t *testing.T, name string) corev1.PodSpec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var spec corev1.PodSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestPodResources(t *testing.T) {
	tests := []struct {
		fixture    string
		cpuRequest string
		memRequest string
		cpuLimit   string
		memLimit   string
		qos        QoS
		partialCPU bool
		partialMem bool
	}{
		{
			fixture:    "single-container.yaml",
			cpuRequest: "100m", memRequest: "128Mi",
			cpuLimit: "500m", memLimit: "512Mi",
			qos: QoSBurstable,
		},
		{
			fixture:    "multi-container.yaml",
			cpuRequest: "150m", memRequest: "192Mi",
			cpuLimit: "600m", memLimit: "640Mi",
			qos: QoSBurstable,
		},
		{
			fixture:    "no-requests.yaml",
			cpuRequest: "0", memRequest: "0",
			cpuLimit: "0", memLimit: "0",
			qos:        QoSBestEffort,
			partialCPU: true, partialMem: true,
		},
		{
			fixture:    "partial-requests.yaml",
			cpuRequest: "100m", memRequest: "192Mi",
			cpuLimit: "0", memLimit: "0",
			qos:        QoSBurstable,
			partialCPU: true,
		},
		{
			fixture:    "init-container.yaml",
			cpuRequest: "2", memRequest: "1Gi",
			cpuLimit: "0", memLimit: "0",
			qos: QoSBurstable,
		},
		{
			fixture:    "init-smaller-than-app.yaml",
			cpuRequest: "300m", memRequest: "384Mi",
			cpuLimit: "0", memLimit: "0",
			qos: QoSBurstable,
		},
		{
			fixture:    "native-sidecar.yaml",
			cpuRequest: "2", memRequest: "1Gi",
			cpuLimit: "0", memLimit: "0",
			qos: QoSBurstable,
		},
		{
			fixture:    "guaranteed.yaml",
			cpuRequest: "500m", memRequest: "512Mi",
			cpuLimit: "500m", memLimit: "512Mi",
			qos: QoSGuaranteed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := podResources(loadSpec(t, tt.fixture))

			eq := func(label string, got resource.Quantity, want string) {
				t.Helper()
				w := resource.MustParse(want)
				if got.Cmp(w) != 0 {
					t.Errorf("%s = %s, want %s", label, got.String(), want)
				}
			}
			eq("cpu request", got.CPURequest, tt.cpuRequest)
			eq("mem request", got.MemRequest, tt.memRequest)
			eq("cpu limit", got.CPULimit, tt.cpuLimit)
			eq("mem limit", got.MemLimit, tt.memLimit)

			if got.QoS != tt.qos {
				t.Errorf("qos = %s, want %s", got.QoS, tt.qos)
			}
			if got.PartialCPURequest != tt.partialCPU {
				t.Errorf("partial cpu = %v, want %v", got.PartialCPURequest, tt.partialCPU)
			}
			if got.PartialMemRequest != tt.partialMem {
				t.Errorf("partial mem = %v, want %v", got.PartialMemRequest, tt.partialMem)
			}
		})
	}
}

func TestQuantityFormatPreserved(t *testing.T) {
	got := podResources(loadSpec(t, "single-container.yaml"))
	if s := got.MemRequest.String(); s != "128Mi" {
		t.Errorf("mem request rendered as %q, want %q", s, "128Mi")
	}
	if s := got.CPURequest.String(); s != "100m" {
		t.Errorf("cpu request rendered as %q, want %q", s, "100m")
	}
}
