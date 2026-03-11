package xray

import (
"context"
_ "embed"
"os"
"path/filepath"
"strings"
"testing"

"github.com/Goalt/personal-server/internal/config"
"github.com/Goalt/personal-server/internal/logger"
corev1 "k8s.io/api/core/v1"
)

// testUUID is a fixed UUID used for unit tests and testdata snapshot files.
const testUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func TestXrayModule_Name(t *testing.T) {
module := &XrayModule{}
if module.Name() != "xray" {
t.Errorf("Name() = %s, want xray", module.Name())
}
}

func TestXrayModule_Prepare(t *testing.T) {
tests := []struct {
name      string
namespace string
}{
{name: "default configuration", namespace: "infra"},
{name: "custom namespace", namespace: "vpn"},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: tt.namespace,
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

secret, configMap, service, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}
if secret == nil {
t.Fatal("prepare() returned nil Secret")
}
if configMap == nil {
t.Fatal("prepare() returned nil ConfigMap")
}
if service == nil {
t.Fatal("prepare() returned nil Service")
}
if deployment == nil {
t.Fatal("prepare() returned nil Deployment")
}

if secret.Namespace != tt.namespace {
t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
}
if configMap.Namespace != tt.namespace {
t.Errorf("ConfigMap namespace = %s, want %s", configMap.Namespace, tt.namespace)
}
if service.Namespace != tt.namespace {
t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
}
if deployment.Namespace != tt.namespace {
t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
}
})
}
}

func TestXrayModule_PrepareRequiresUUID(t *testing.T) {
tests := []struct {
name    string
secrets map[string]string
}{
{name: "missing uuid key", secrets: map[string]string{}},
{name: "empty uuid value", secrets: map[string]string{"xray_uuid": ""}},
{name: "nil secrets map", secrets: nil},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   tt.secrets,
},
}

_, _, _, _, err := module.prepare()
if err == nil {
t.Error("prepare() should return an error when xray_uuid is not configured")
}
if !strings.Contains(err.Error(), "xray_uuid") {
t.Errorf("error message should mention xray_uuid, got: %v", err)
}
})
}
}

func TestXrayModule_PrepareSecret(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

secret, _, _, _, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if secret.Name != secretName {
t.Errorf("Secret name = %s, want %s", secret.Name, secretName)
}
if secret.Type != corev1.SecretTypeOpaque {
t.Errorf("Secret type = %s, want Opaque", secret.Type)
}
expectedLabels := map[string]string{"app": "xray", "managed-by": "personal-server"}
for key, want := range expectedLabels {
if got, ok := secret.Labels[key]; !ok || got != want {
t.Errorf("Secret label %s = %s, want %s", key, got, want)
}
}
if secret.StringData["xray_uuid"] != testUUID {
t.Errorf("Secret xray_uuid = %s, want %s", secret.StringData["xray_uuid"], testUUID)
}
}

func TestXrayModule_PrepareConfigMap(t *testing.T) {
wsPath := "/my-vpn-path"
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets: map[string]string{
"xray_uuid":           testUUID,
"xray_websocket_path": wsPath,
},
},
}

_, configMap, _, _, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if configMap.Name != configMapName {
t.Errorf("ConfigMap name = %s, want %s", configMap.Name, configMapName)
}
expectedLabels := map[string]string{"app": "xray", "managed-by": "personal-server"}
for key, want := range expectedLabels {
if got, ok := configMap.Labels[key]; !ok || got != want {
t.Errorf("ConfigMap label %s = %s, want %s", key, got, want)
}
}
cfgJSON, ok := configMap.Data["config.json"]
if !ok {
t.Fatal("ConfigMap missing config.json key")
}
if !strings.Contains(cfgJSON, testUUID) {
t.Errorf("config.json does not contain UUID %s", testUUID)
}
if !strings.Contains(cfgJSON, wsPath) {
t.Errorf("config.json does not contain websocket path %s", wsPath)
}
if !strings.Contains(cfgJSON, "vless") {
t.Error("config.json missing protocol vless")
}
if !strings.Contains(cfgJSON, `"network": "ws"`) {
t.Error("config.json missing websocket network setting")
}
}

func TestXrayModule_PrepareConfigMapDefaultPath(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, configMap, _, _, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

cfgJSON := configMap.Data["config.json"]
if !strings.Contains(cfgJSON, defaultWsPath) {
t.Errorf("config.json should contain default WebSocket path %s", defaultWsPath)
}
}

func TestXrayModule_PrepareService(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, _, service, _, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if service.Name != serviceName {
t.Errorf("Service name = %s, want %s", service.Name, serviceName)
}
if service.Spec.Type != corev1.ServiceTypeClusterIP {
t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
}
expectedLabels := map[string]string{"app": "xray", "managed-by": "personal-server"}
for key, want := range expectedLabels {
if got, ok := service.Labels[key]; !ok || got != want {
t.Errorf("Service label %s = %s, want %s", key, got, want)
}
}
if service.Spec.Selector["app"] != "xray" {
t.Errorf("Service selector app = %s, want xray", service.Spec.Selector["app"])
}
if len(service.Spec.Ports) != 1 {
t.Fatalf("Service ports count = %d, want 1", len(service.Spec.Ports))
}
port := service.Spec.Ports[0]
if port.Name != "ws" {
t.Errorf("Service port name = %s, want ws", port.Name)
}
if port.Port != containerPort {
t.Errorf("Service port = %d, want %d", port.Port, containerPort)
}
if port.TargetPort.IntVal != containerPort {
t.Errorf("Service targetPort = %d, want %d", port.TargetPort.IntVal, containerPort)
}
if port.Protocol != corev1.ProtocolTCP {
t.Errorf("Service port protocol = %s, want TCP", port.Protocol)
}
}

func TestXrayModule_PrepareDeployment(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, _, _, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if deployment.Name != deploymentName {
t.Errorf("Deployment name = %s, want %s", deployment.Name, deploymentName)
}
expectedLabels := map[string]string{"app": "xray", "managed-by": "personal-server"}
for key, want := range expectedLabels {
if got, ok := deployment.Labels[key]; !ok || got != want {
t.Errorf("Deployment label %s = %s, want %s", key, got, want)
}
}
if deployment.Spec.Replicas == nil {
t.Fatal("Deployment replicas is nil")
}
if *deployment.Spec.Replicas != 1 {
t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
}
if deployment.Spec.Selector.MatchLabels["app"] != "xray" {
t.Errorf("Deployment selector app = %s, want xray", deployment.Spec.Selector.MatchLabels["app"])
}
if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
}
}

func TestXrayModule_PrepareDeploymentContainer(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, _, _, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if len(deployment.Spec.Template.Spec.Containers) != 1 {
t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
}
container := deployment.Spec.Template.Spec.Containers[0]

if container.Name != "xray" {
t.Errorf("Container name = %s, want xray", container.Name)
}
if container.Image != defaultImage {
t.Errorf("Container image = %s, want %s", container.Image, defaultImage)
}
if len(container.Ports) != 1 {
t.Fatalf("Container ports count = %d, want 1", len(container.Ports))
}
if container.Ports[0].ContainerPort != containerPort {
t.Errorf("Container port = %d, want %d", container.Ports[0].ContainerPort, containerPort)
}
if len(container.VolumeMounts) != 1 {
t.Fatalf("Container volume mounts count = %d, want 1", len(container.VolumeMounts))
}
vm := container.VolumeMounts[0]
if vm.Name != "xray-config" {
t.Errorf("VolumeMount name = %s, want xray-config", vm.Name)
}
if vm.MountPath != "/etc/xray" {
t.Errorf("VolumeMount mountPath = %s, want /etc/xray", vm.MountPath)
}
if !vm.ReadOnly {
t.Error("VolumeMount should be read-only")
}
}

func TestXrayModule_PrepareDeploymentSecurityContext(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, _, _, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}
container := deployment.Spec.Template.Spec.Containers[0]

if container.SecurityContext == nil {
t.Fatal("Container SecurityContext is nil")
}
sc := container.SecurityContext
if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
t.Error("AllowPrivilegeEscalation should be false")
}
if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
t.Error("RunAsNonRoot should be true")
}
if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
t.Error("ReadOnlyRootFilesystem should be true")
}
if sc.Capabilities == nil {
t.Fatal("Capabilities is nil")
}
if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
t.Errorf("Capabilities.Drop = %v, want [ALL]", sc.Capabilities.Drop)
}
}

func TestXrayModule_PrepareDeploymentVolumes(t *testing.T) {
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
}

_, _, _, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}

if len(deployment.Spec.Template.Spec.Volumes) != 1 {
t.Fatalf("Volumes count = %d, want 1", len(deployment.Spec.Template.Spec.Volumes))
}
vol := deployment.Spec.Template.Spec.Volumes[0]
if vol.Name != "xray-config" {
t.Errorf("Volume name = %s, want xray-config", vol.Name)
}
if vol.ConfigMap == nil {
t.Fatal("Volume should use ConfigMap")
}
if vol.ConfigMap.Name != configMapName {
t.Errorf("Volume ConfigMap name = %s, want %s", vol.ConfigMap.Name, configMapName)
}
}

func TestXrayModule_CustomImage(t *testing.T) {
customImage := "ghcr.io/xtls/xray-core:v1.8.0"
module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "test-namespace",
Secrets: map[string]string{
"xray_uuid":  testUUID,
"xray_image": customImage,
},
},
}

_, _, _, deployment, err := module.prepare()
if err != nil {
t.Fatalf("prepare() returned unexpected error: %v", err)
}
if deployment.Spec.Template.Spec.Containers[0].Image != customImage {
t.Errorf("Container image = %s, want %s", deployment.Spec.Template.Spec.Containers[0].Image, customImage)
}
}

//go:embed testdata/secret.yaml
var expectedSecretYAML string

//go:embed testdata/configmap.yaml
var expectedConfigmapYAML string

//go:embed testdata/service.yaml
var expectedServiceYAML string

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

func TestXrayModule_Generate(t *testing.T) {
tempDir := t.TempDir()
originalWd, err := os.Getwd()
if err != nil {
t.Fatalf("failed to get working directory: %v", err)
}
if err := os.Chdir(tempDir); err != nil {
t.Fatalf("failed to change to temp directory: %v", err)
}
defer os.Chdir(originalWd)

module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "infra",
Secrets:   map[string]string{"xray_uuid": testUUID},
},
log: logger.Default(),
}

ctx := context.Background()
if err := module.Generate(ctx); err != nil {
t.Fatalf("Generate() failed: %v", err)
}

testCases := []struct {
name     string
filename string
expected string
}{
{"secret", "configs/xray/secret.yaml", expectedSecretYAML},
{"configmap", "configs/xray/configmap.yaml", expectedConfigmapYAML},
{"service", "configs/xray/service.yaml", expectedServiceYAML},
{"deployment", "configs/xray/deployment.yaml", expectedDeploymentYAML},
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
generatedPath := filepath.Join(tempDir, tc.filename)
generatedContent, err := os.ReadFile(generatedPath)
if err != nil {
t.Fatalf("failed to read generated file %s: %v", tc.filename, err)
}
if string(generatedContent) != tc.expected {
t.Errorf("Generated YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s",
string(generatedContent), tc.expected)
}
})
}
}

func TestXrayModule_GenerateMissingUUID(t *testing.T) {
tempDir := t.TempDir()
originalWd, _ := os.Getwd()
os.Chdir(tempDir)
defer os.Chdir(originalWd)

module := &XrayModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name:      "xray",
Namespace: "infra",
Secrets:   map[string]string{},
},
log: logger.Default(),
}

err := module.Generate(context.Background())
if err == nil {
t.Error("Generate() should return an error when xray_uuid is not configured")
}
}
