package config

import (
	"testing"
)

func TestGetIngress(t *testing.T) {
	cfg := &Config{
		Ingresses: []IngressConfig{
			{
				Name:      "web-ingress",
				Namespace: "default",
				Rules: []IngressRule{
					{
						Host:        "example.com",
						Path:        "/",
						ServiceName: "web-service",
						ServicePort: 80,
					},
				},
			},
			{
				Name:      "api-ingress",
				Namespace: "api",
				Rules: []IngressRule{
					{
						Host:        "api.example.com",
						Path:        "/",
						ServiceName: "api-service",
						ServicePort: 8080,
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		ingressName string
		wantErr     bool
		wantName    string
	}{
		{
			name:        "existing ingress",
			ingressName: "web-ingress",
			wantErr:     false,
			wantName:    "web-ingress",
		},
		{
			name:        "another existing ingress",
			ingressName: "api-ingress",
			wantErr:     false,
			wantName:    "api-ingress",
		},
		{
			name:        "non-existent ingress",
			ingressName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress, err := cfg.GetIngress(tt.ingressName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIngress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ingress.Name != tt.wantName {
				t.Errorf("GetIngress() name = %v, want %v", ingress.Name, tt.wantName)
			}
		})
	}
}
