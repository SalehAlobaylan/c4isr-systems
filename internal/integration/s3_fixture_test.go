package integration

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// S3Fixture is a disposable actual S3-compatible server, not an in-memory port.
// Static keys belong only to this isolated container and carry no real access.
func newS3Fixture(t *testing.T) *artifacts.S3 {
	t.Helper()
	if os.Getenv("C4ISR_TEST_INTEGRATION") != "1" {
		t.Skip("set C4ISR_TEST_INTEGRATION=1 for disposable S3 integration")
	}
	cfg := `metadata_dir = "/tmp/meta"
data_dir = "/tmp/data"
db_engine = "sqlite"
replication_factor = 1
rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "` + strings.Repeat("a", 64) + `"
[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
[admin]
api_bind_addr = "[::]:3903"
admin_token = "test-administration-token"
`
	file := filepath.Join(t.TempDir(), "garage.toml")
	if e := os.WriteFile(file, []byte(cfg), 0600); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	c, e := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "dxflrs/garage:v2.4.1@sha256:9c96caa2612d3411acc5b0e6701fb238dbfba33e533a6d7d3d811a4b12d0d020", Cmd: []string{"/garage", "server", "--single-node", "--default-bucket"}, Env: map[string]string{"GARAGE_DEFAULT_ACCESS_KEY": "GK11111111111111111111111111111111", "GARAGE_DEFAULT_SECRET_KEY": strings.Repeat("b", 64), "GARAGE_DEFAULT_BUCKET": "emad-fixtures"}, ExposedPorts: []string{"3900/tcp"}, Files: []testcontainers.ContainerFile{{HostFilePath: file, ContainerFilePath: "/etc/garage.toml", FileMode: 0600}}, WaitingFor: wait.ForListeningPort("3900/tcp").WithStartupTimeout(60 * time.Second)}, Started: true})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	host, e := c.Host(ctx)
	if e != nil {
		t.Fatal(e)
	}
	port, e := c.MappedPort(ctx, "3900/tcp")
	if e != nil {
		t.Fatal(e)
	}
	s, e := artifacts.NewS3(artifacts.S3Config{Endpoint: "http://" + host + ":" + port.Port(), Bucket: "emad-fixtures", Region: "garage", AccessKey: "GK11111111111111111111111111111111", SecretKey: strings.Repeat("b", 64), AllowHTTP: true})
	if e != nil {
		t.Fatal(e)
	}
	// Listener readiness can precede default key/bucket initialization.
	for deadline := time.Now().Add(10 * time.Second); ; {
		e = s.Put(context.Background(), "fixture-readiness/"+artifacts.Hash([]byte("ready")), []byte("ready"), "application/json")
		if e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(e)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return s
}
