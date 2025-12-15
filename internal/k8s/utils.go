package k8s

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// FormatAge formats a duration into a human-readable age string
func FormatAge(d time.Duration) string {
	if d.Hours() >= 24 {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d"
		}
		return fmt.Sprintf("%dd", days)
	}
	if d.Hours() >= 1 {
		hours := int(d.Hours())
		if hours == 1 {
			return "1h"
		}
		return fmt.Sprintf("%dh", hours)
	}
	if d.Minutes() >= 1 {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1m"
		}
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// Int32Ptr is a helper function to convert int32 to *int32
func Int32Ptr(i int32) *int32 {
	return &i
}

// GetMapKeys returns the keys of a map as a slice
func GetMapKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetSecretOrDefault returns the secret value if it exists, otherwise returns the default value
func GetSecretOrDefault(secrets map[string]string, key, defaultValue string) string {
	if value, exists := secrets[key]; exists {
		return value
	}
	return defaultValue
}

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool {
	return &b
}

// YAMLToJSON converts YAML content to JSON format
func YAMLToJSON(yamlContent string) (string, error) {
	var data interface{}

	if err := yaml.Unmarshal([]byte(yamlContent), &data); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert map[string]interface{} keys since yaml.v3 uses string keys by default
	data = convertMapKeys(data)

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// JSONToYAML converts JSON content to YAML format
func JSONToYAML(jsonContent string) (string, error) {
	var data interface{}

	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// convertMapKeys recursively converts map keys to ensure JSON compatibility
func convertMapKeys(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{})
		for k, val := range x {
			m[k] = convertMapKeys(val)
		}
		return m
	case []interface{}:
		for i, val := range x {
			x[i] = convertMapKeys(val)
		}
		return x
	default:
		return v
	}
}
