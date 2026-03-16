package k8s

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// DefaultImagePullPolicy matches Kubernetes defaulting for container images.
func DefaultImagePullPolicy(image string) corev1.PullPolicy {
	last := image
	if idx := strings.LastIndex(last, "/"); idx >= 0 {
		last = last[idx+1:]
	}

	if strings.Contains(last, "@") {
		return corev1.PullIfNotPresent
	}

	if !strings.Contains(last, ":") || strings.HasSuffix(last, ":latest") {
		return corev1.PullAlways
	}

	return corev1.PullIfNotPresent
}
