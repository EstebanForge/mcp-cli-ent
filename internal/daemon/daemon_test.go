package daemon

// Tests for daemon session lifecycle: a failed startup health check must
// evict the session so the next call recreates it, instead of leaving a
// zombie session with a nil client in the map until daemon restart.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mcp-cli-ent/mcp-cli/internal/config"
	"github.com/mcp-cli-ent/mcp-cli/internal/mcp"
)

type fakeClient struct {
	failTools bool
	calls     atomic.Int32
	closed    atomic.Bool
}

func (f *fakeClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	f.calls.Add(1)
	if f.failTools {
		return nil, errors.New("stub tools failure")
	}
	return []mcp.Tool{{Name: "ok"}}, nil
}

func (f *fakeClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.ToolResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) Initialize(ctx context.Context, params *mcp.InitializeParams) (*mcp.InitializeResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) CreateMessage(ctx context.Context, request *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) RequestInput(ctx context.Context, params *mcp.RequestInputParams) (*mcp.RequestInputResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) ListRoots(ctx context.Context) ([]mcp.Root, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeClient) NotifyRootsListChanged(roots []mcp.Root) error {
	return nil
}

func (f *fakeClient) Close() error {
	f.closed.Store(true)
	return nil
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %s", timeout, msg)
}

// TestFailedHealthCheckEvictsSession: when the startup health check fails,
// the session must be removed from the daemon's map so the next call can
// recreate it. Break it guards: pre-fix, the failed session stayed in the
// map with a nil client forever, blocking a clean recreation until the
// daemon restarted.
func TestFailedHealthCheckEvictsSession(t *testing.T) {
	d, err := NewDaemon(DefaultDaemonConfig())
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	bad := &fakeClient{failTools: true}
	d.clientFactory = func(cfg config.ServerConfig) (mcp.MCPClient, error) {
		return bad, nil
	}

	if err := d.StartSession("bad", config.ServerConfig{Command: "stub"}); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Eviction is asserted on the map directly: GetSession errors for any
	// non-Active status, so it cannot distinguish eviction from a zombie
	// session stuck in Starting/Error.
	waitFor(t, 3*time.Second,
		func() bool {
			d.sessionMutex.RLock()
			_, exists := d.sessions["bad"]
			d.sessionMutex.RUnlock()
			return !exists
		},
		"failed session was not evicted from the daemon map",
	)

	if !bad.closed.Load() {
		t.Errorf("failed client was not closed after health-check failure")
	}

	// Recreation path: with a working client the next StartSession must be
	// able to bring the server back to Active.
	good := &fakeClient{}
	d.clientFactory = func(cfg config.ServerConfig) (mcp.MCPClient, error) {
		return good, nil
	}
	if err := d.StartSession("bad", config.ServerConfig{Command: "stub"}); err != nil {
		t.Fatalf("StartSession after eviction: %v", err)
	}

	waitFor(t, 3*time.Second,
		func() bool {
			s, err := d.GetSession("bad")
			return err == nil && s.Status == SessionStatusActive && s.Client != nil
		},
		"session did not reach Active after recreation",
	)
}
