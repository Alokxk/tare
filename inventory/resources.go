package inventory

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type QoS string

const (
	QoSGuaranteed QoS = "Guaranteed"
	QoSBurstable  QoS = "Burstable"
	QoSBestEffort QoS = "BestEffort"
)

// Resources is one pod's effective footprint, as the scheduler reserves it.
type Resources struct {
	CPURequest resource.Quantity
	MemRequest resource.Quantity
	CPULimit   resource.Quantity
	MemLimit   resource.Quantity
	QoS        QoS

	// Set when a container that runs for the pod's lifetime omits that
	// request. The totals above are then a floor rather than the real figure,
	// so any ratio built on them understates. Reported instead of silently
	// counted as zero.
	PartialCPURequest bool
	PartialMemRequest bool
}

// A native sidecar is an init container that never exits, so it runs alongside
// the app containers rather than before them.
func isSidecar(c corev1.Container) bool {
	return c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
}

// podResources computes effective = max(sum of lifetime containers, largest
// single init container). Ordinary init containers run one at a time and exit
// before the app starts, so the pod never holds their total at once; summing
// them in would invent overprovisioning that is not there.
func podResources(spec corev1.PodSpec) Resources {
	sumReq, sumLim := corev1.ResourceList{}, corev1.ResourceList{}
	maxInitReq, maxInitLim := corev1.ResourceList{}, corev1.ResourceList{}

	for _, c := range spec.InitContainers {
		if isSidecar(c) {
			addInto(sumReq, c.Resources.Requests)
			addInto(sumLim, c.Resources.Limits)
			continue
		}
		maxInto(maxInitReq, c.Resources.Requests)
		maxInto(maxInitLim, c.Resources.Limits)
	}
	for _, c := range spec.Containers {
		addInto(sumReq, c.Resources.Requests)
		addInto(sumLim, c.Resources.Limits)
	}

	maxInto(sumReq, maxInitReq)
	maxInto(sumLim, maxInitLim)

	return Resources{
		CPURequest:        sumReq[corev1.ResourceCPU],
		MemRequest:        sumReq[corev1.ResourceMemory],
		CPULimit:          sumLim[corev1.ResourceCPU],
		MemLimit:          sumLim[corev1.ResourceMemory],
		QoS:               qosClass(spec),
		PartialCPURequest: missingRequest(spec, corev1.ResourceCPU),
		PartialMemRequest: missingRequest(spec, corev1.ResourceMemory),
	}
}

// Quantity.Add is exact. Converting to float64 to sum would lose both
// precision and the original unit, printing 0.30000000000000004 cores.
func addInto(dst, src corev1.ResourceList) {
	for name, q := range src {
		cur := dst[name]
		cur.Add(q)
		dst[name] = cur
	}
}

func maxInto(dst, src corev1.ResourceList) {
	for name, q := range src {
		if cur, ok := dst[name]; !ok || cur.Cmp(q) < 0 {
			dst[name] = q
		}
	}
}

// Only containers alive for the pod's lifetime matter here: an init container
// that exits before the app starts cannot understate the steady-state request.
func missingRequest(spec corev1.PodSpec, name corev1.ResourceName) bool {
	for _, c := range spec.Containers {
		if _, ok := c.Resources.Requests[name]; !ok {
			return true
		}
	}
	for _, c := range spec.InitContainers {
		if !isSidecar(c) {
			continue
		}
		if _, ok := c.Resources.Requests[name]; !ok {
			return true
		}
	}
	return false
}

// Mirrors the API server's classification. Every container counts, init
// containers included, because a pod that sets nothing anywhere is BestEffort
// regardless of which phase the containers belong to.
func qosClass(spec corev1.PodSpec) QoS {
	anySet, guaranteed := false, true

	for _, c := range append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...) {
		for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
			req, hasReq := c.Resources.Requests[name]
			lim, hasLim := c.Resources.Limits[name]

			if (hasReq && !req.IsZero()) || (hasLim && !lim.IsZero()) {
				anySet = true
			}
			if !hasLim || lim.IsZero() {
				guaranteed = false
				continue
			}
			// The API server defaults an absent request to the limit, so an
			// unset request here is still Guaranteed.
			if hasReq && req.Cmp(lim) != 0 {
				guaranteed = false
			}
		}
	}

	switch {
	case !anySet:
		return QoSBestEffort
	case guaranteed:
		return QoSGuaranteed
	default:
		return QoSBurstable
	}
}
