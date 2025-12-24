package cloudflare

import (
"context"
"os"
"testing"

"github.com/Goalt/personal-server/internal/config"
"github.com/Goalt/personal-server/internal/logger"
)

func TestGenFixtures(t *testing.T) {
if os.Getenv("GEN_FIXTURES") != "1" {
t.Skip("Skipping fixture generation")
}

tempDir := "/tmp/cloudflare-fixtures"
os.RemoveAll(tempDir)
os.MkdirAll(tempDir, 0755)
originalWd, _ := os.Getwd()
os.Chdir(tempDir)
defer os.Chdir(originalWd)

module := &CloudflareModule{
GeneralConfig: config.GeneralConfig{Domain: "example.com"},
ModuleConfig: config.Module{
Name: "cloudflare",
Namespace: "infra",
Secrets: map[string]string{"cloudflare_api_token": "test-token-123"},
},
log: logger.Default(),
}

if err := module.Generate(context.Background()); err != nil {
t.Fatal(err)
}

t.Logf("Generated files in: %s/configs/cloudflare/", tempDir)
}
