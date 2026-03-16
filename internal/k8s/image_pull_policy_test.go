package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestDefaultImagePullPolicy(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  corev1.PullPolicy
	}{
		{
			name:  "no tag defaults to always",
			image: "nginx",
			want:  corev1.PullAlways,
		},
		{
			name:  "latest tag defaults to always",
			image: "ghcr.io/hacdias/webdav:latest",
			want:  corev1.PullAlways,
		},
		{
			name:  "explicit tag defaults to if not present",
			image: "ghcr.io/goalt/work-config:sha-942241f",
			want:  corev1.PullIfNotPresent,
		},
		{
			name:  "registry port does not imply latest",
			image: "registry.example.com:5000/myapp:1.2.3",
			want:  corev1.PullIfNotPresent,
		},
		{
			name:  "digest defaults to if not present",
			image: "registry.example.com/myapp@sha256:deadbeef",
			want:  corev1.PullIfNotPresent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultImagePullPolicy(tt.image); got != tt.want {
				t.Errorf("DefaultImagePullPolicy(%q) = %s, want %s", tt.image, got, tt.want)
			}
		})
	}
}
