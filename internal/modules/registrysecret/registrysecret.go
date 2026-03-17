package registrysecret

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistrySecretModule manages Kubernetes docker-registry secrets for the
// top-level registries defined in the configuration.
type RegistrySecretModule struct {
	Registries map[string]config.RegistryCredentials
	log        logger.Logger
}

// New creates a new RegistrySecretModule.
func New(registries map[string]config.RegistryCredentials, log logger.Logger) *RegistrySecretModule {
	return &RegistrySecretModule{
		Registries: registries,
		log:        log,
	}
}

func (m *RegistrySecretModule) Name() string {
	return "registry"
}

// Generate writes a Kubernetes Secret YAML file for each configured registry.
func (m *RegistrySecretModule) Generate(ctx context.Context) error {
	if len(m.Registries) == 0 {
		m.log.Info("No registries configured\n")
		return nil
	}

	outputDir := filepath.Join("configs", "registry")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating registry secret configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	count := 0
	for name, creds := range m.Registries {
		secret, err := m.prepareSecret(name, creds)
		if err != nil {
			return fmt.Errorf("preparing secret for registry %q: %w", name, err)
		}

		jsonBytes, err := json.Marshal(secret)
		if err != nil {
			return fmt.Errorf("failed to convert secret for registry %q to JSON: %w", name, err)
		}

		yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to convert secret for registry %q to YAML: %w", name, err)
		}

		filename := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", name))
		if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("failed to write secret for registry %q to file: %w", name, err)
		}

		m.log.Success("Generated: %s\n", filename)
		count++
	}

	m.log.Info("\nCompleted: %d/%d registry secret configurations generated successfully\n", count, len(m.Registries))
	return nil
}

// Apply creates or updates a Kubernetes docker-registry Secret for each configured registry.
func (m *RegistrySecretModule) Apply(ctx context.Context) error {
	if len(m.Registries) == 0 {
		m.log.Info("No registries configured\n")
		return nil
	}

	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying registry secret configurations...\n\n")

	for name, creds := range m.Registries {
		if creds.Namespace == "" {
			m.log.Warn("Registry %q has no namespace configured, skipping\n", name)
			continue
		}

		secret, err := m.prepareSecret(name, creds)
		if err != nil {
			return fmt.Errorf("preparing secret for registry %q: %w", name, err)
		}

		m.log.Progress("Applying Secret: %s (namespace: %s)\n", name, creds.Namespace)
		_, err = clientset.CoreV1().Secrets(creds.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().Secrets(creds.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
		}
		if err != nil {
			return fmt.Errorf("failed to create or update secret for registry %q: %w", name, err)
		}
		m.log.Success("Applied Secret: %s in namespace %s\n", name, creds.Namespace)
	}

	m.log.Info("\nCompleted: registry secrets applied successfully\n")
	return nil
}

// Clean deletes the Kubernetes Secret for each configured registry.
func (m *RegistrySecretModule) Clean(ctx context.Context) error {
	if len(m.Registries) == 0 {
		m.log.Info("No registries configured\n")
		return nil
	}

	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning registry secrets...\n\n")

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	for name, creds := range m.Registries {
		if creds.Namespace == "" {
			m.log.Warn("Registry %q has no namespace configured, skipping\n", name)
			continue
		}

		m.log.Info("🗑️  Deleting Secret: %s (namespace: %s)\n", name, creds.Namespace)
		err := clientset.CoreV1().Secrets(creds.Namespace).Delete(ctx, name, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("Secret %q not found (already deleted or never existed)\n", name)
			} else {
				m.log.Error("Failed to delete secret %q: %v\n", name, err)
				return err
			}
		} else {
			m.log.Success("Deleted Secret: %s\n", name)
		}
	}

	m.log.Info("\nCompleted: registry secrets deleted successfully\n")
	return nil
}

// Status checks and prints the status of each configured registry secret.
func (m *RegistrySecretModule) Status(ctx context.Context) error {
	if len(m.Registries) == 0 {
		m.log.Info("No registries configured\n")
		return nil
	}

	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking registry secrets...\n\n")

	for name, creds := range m.Registries {
		if creds.Namespace == "" {
			m.log.Warn("Registry %q has no namespace configured\n", name)
			continue
		}

		secret, err := clientset.CoreV1().Secrets(creds.Namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Error("Secret %q not found in namespace %s\n", name, creds.Namespace)
			} else {
				m.log.Error("Error checking secret %q: %v\n", name, err)
			}
		} else {
			age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
			m.log.Success("Secret %q in namespace %s\n", name, creds.Namespace)
			m.log.Info("   Server: %s\n", creds.Server)
			m.log.Info("   Type: %s\n", secret.Type)
			m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		}
	}

	return nil
}

// prepareSecret builds a Kubernetes docker-registry Secret for the given registry key and credentials.
func (m *RegistrySecretModule) prepareSecret(name string, creds config.RegistryCredentials) (*corev1.Secret, error) {
	auth := fmt.Sprintf("%s:%s", creds.Username, creds.Password)
	authEncoded := base64.StdEncoding.EncodeToString([]byte(auth))

	authEntry := map[string]interface{}{
		"username": creds.Username,
		"password": creds.Password,
		"auth":     authEncoded,
	}
	if creds.Email != "" {
		authEntry["email"] = creds.Email
	}

	configJSON := map[string]interface{}{
		"auths": map[string]interface{}{
			creds.Server: authEntry,
		},
	}

	jsonBytes, err := json.Marshal(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registry credentials: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: creds.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"type":       "registry-secret",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": jsonBytes,
		},
	}

	return secret, nil
}
