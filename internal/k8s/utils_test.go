package k8s

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestYAMLToJSON(t *testing.T) {
	tests := []struct {
		name        string
		yamlInput   string
		expected    string
		expectError bool
	}{
		{
			name:      "simple key-value",
			yamlInput: "key: value",
			expected:  `{"key":"value"}`,
		},
		{
			name: "nested structure",
			yamlInput: `
parent:
  child: value
  number: 42`,
			expected: `{"parent":{"child":"value","number":42}}`,
		},
		{
			name: "array",
			yamlInput: `
items:
  - first
  - second
  - third`,
			expected: `{"items":["first","second","third"]}`,
		},
		{
			name: "mixed types",
			yamlInput: `
string: hello
number: 123
float: 3.14
boolean: true
null_value: null`,
			expected: `{"boolean":true,"float":3.14,"null_value":null,"number":123,"string":"hello"}`,
		},
		{
			name: "complex nested structure",
			yamlInput: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  labels:
    app: myapp
data:
  config.json: |
    {"setting": "value"}`,
			expected: `{"apiVersion":"v1","data":{"config.json":"{\"setting\": \"value\"}"},"kind":"ConfigMap","metadata":{"labels":{"app":"myapp"},"name":"test-config"}}`,
		},
		{
			name: "array of objects",
			yamlInput: `
users:
  - name: alice
    age: 30
  - name: bob
    age: 25`,
			expected: `{"users":[{"age":30,"name":"alice"},{"age":25,"name":"bob"}]}`,
		},
		{
			name:      "empty yaml",
			yamlInput: "",
			expected:  "null",
		},
		{
			name:        "invalid yaml",
			yamlInput:   "key: [invalid",
			expectError: true,
		},
		{
			name: "multiline string",
			yamlInput: `
description: |
  This is a
  multiline string`,
			expected: `{"description":"This is a\nmultiline string"}`,
		},
		{
			name: "inline array",
			yamlInput: `
tags: [one, two, three]`,
			expected: `{"tags":["one","two","three"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := YAMLToJSON(tt.yamlInput)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Compare JSON by parsing both and comparing the structures
			// This handles key ordering differences
			var resultData, expectedData interface{}
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Errorf("failed to parse result JSON: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedData); err != nil {
				t.Errorf("failed to parse expected JSON: %v", err)
				return
			}

			// Re-marshal both for consistent comparison
			resultJSON, _ := json.Marshal(resultData)
			expectedJSON, _ := json.Marshal(expectedData)

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("YAMLToJSON() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestJSONToYAML(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expected    string
		expectError bool
	}{
		{
			name:      "simple key-value",
			jsonInput: `{"key":"value"}`,
			expected:  "key: value\n",
		},
		{
			name:      "nested structure",
			jsonInput: `{"parent":{"child":"value","number":42}}`,
			expected:  "parent:\n    child: value\n    number: 42\n",
		},
		{
			name:      "array",
			jsonInput: `{"items":["first","second","third"]}`,
			expected:  "items:\n    - first\n    - second\n    - third\n",
		},
		{
			name:      "mixed types",
			jsonInput: `{"boolean":true,"float":3.14,"null_value":null,"number":123,"string":"hello"}`,
			expected:  "boolean: true\nfloat: 3.14\nnull_value: null\nnumber: 123\nstring: hello\n",
		},
		{
			name:      "array of objects",
			jsonInput: `{"users":[{"age":30,"name":"alice"},{"age":25,"name":"bob"}]}`,
			expected:  "users:\n    - age: 30\n      name: alice\n    - age: 25\n      name: bob\n",
		},
		{
			name:      "null json",
			jsonInput: "null",
			expected:  "null\n",
		},
		{
			name:        "invalid json",
			jsonInput:   `{"key": invalid}`,
			expectError: true,
		},
		{
			name:      "empty object",
			jsonInput: `{}`,
			expected:  "{}\n",
		},
		{
			name:      "empty array",
			jsonInput: `[]`,
			expected:  "[]\n",
		},
		{
			name:      "string with newlines",
			jsonInput: `{"description":"line1\nline2\nline3"}`,
			expected:  "description: |-\n    line1\n    line2\n    line3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONToYAML(tt.jsonInput)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Compare YAML by parsing both and comparing the structures
			// This handles formatting differences
			var resultData, expectedData interface{}
			if err := yaml.Unmarshal([]byte(result), &resultData); err != nil {
				t.Errorf("failed to parse result YAML: %v", err)
				return
			}
			if err := yaml.Unmarshal([]byte(tt.expected), &expectedData); err != nil {
				t.Errorf("failed to parse expected YAML: %v", err)
				return
			}

			// Re-marshal both as JSON for consistent comparison
			resultJSON, _ := json.Marshal(resultData)
			expectedJSON, _ := json.Marshal(expectedData)

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("JSONToYAML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "multiple minutes",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "one hour",
			duration: 1 * time.Hour,
			expected: "1h",
		},
		{
			name:     "multiple hours",
			duration: 5 * time.Hour,
			expected: "5h",
		},
		{
			name:     "one day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "multiple days",
			duration: 72 * time.Hour,
			expected: "3d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAge(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatAge(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestInt32Ptr(t *testing.T) {
	val := int32(42)
	ptr := Int32Ptr(val)

	if ptr == nil {
		t.Fatal("Int32Ptr returned nil")
	}
	if *ptr != val {
		t.Errorf("Int32Ptr(%d) = %d, want %d", val, *ptr, val)
	}
}

func TestBoolPtr(t *testing.T) {
	trueVal := true
	truePtr := BoolPtr(trueVal)
	if truePtr == nil || *truePtr != true {
		t.Error("BoolPtr(true) failed")
	}

	falseVal := false
	falsePtr := BoolPtr(falseVal)
	if falsePtr == nil || *falsePtr != false {
		t.Error("BoolPtr(false) failed")
	}
}

func TestGetMapKeys(t *testing.T) {
	m := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	keys := GetMapKeys(m)

	if len(keys) != 3 {
		t.Errorf("GetMapKeys returned %d keys, want 3", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	for expectedKey := range m {
		if !keySet[expectedKey] {
			t.Errorf("GetMapKeys missing key: %s", expectedKey)
		}
	}
}

func TestGetSecretOrDefault(t *testing.T) {
	secrets := map[string]string{
		"existing": "secret-value",
	}

	// Test existing key
	result := GetSecretOrDefault(secrets, "existing", "default")
	if result != "secret-value" {
		t.Errorf("GetSecretOrDefault for existing key = %s, want secret-value", result)
	}

	// Test non-existing key
	result = GetSecretOrDefault(secrets, "missing", "default")
	if result != "default" {
		t.Errorf("GetSecretOrDefault for missing key = %s, want default", result)
	}
}
