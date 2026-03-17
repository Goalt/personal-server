package k8s

import "k8s.io/client-go/kubernetes"

// KubernetesClient is an alias for the upstream kubernetes.Interface, exposing all
// typed client groups.  It exists so tests can substitute a fake client (via
// k8s.io/client-go/kubernetes/fake) without touching production code paths.
type KubernetesClient = kubernetes.Interface
