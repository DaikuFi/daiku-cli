package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeExecutor struct {
	mu      sync.Mutex
	args    [][]string
	block   bool
	expired bool
}

func (f *fakeExecutor) Commands() []agent.Command {
	common := []agent.Flag{{Name: "json", Type: "bool", Default: "false"}, {Name: "agent", Type: "bool", Default: "false"}}
	return []agent.Command{
		{Name: "list", Path: "daiku transactions list", Use: "daiku transactions list", Short: "List transactions", Runnable: true, ReadOnly: true, Flags: append(common, agent.Flag{Name: "household", Type: "string", Required: true, Usage: "household ID"}), Subcommands: []agent.CommandSummary{}},
		{Name: "create", Path: "daiku transactions create", Use: "daiku transactions create", Short: "Create a transaction", Runnable: true, Flags: append(common, agent.Flag{Name: "household", Type: "string", Required: true, Usage: "household ID"}), Subcommands: []agent.CommandSummary{}},
		{Name: "remove", Path: "daiku profile remove", Use: "daiku profile remove <name>", Short: "Remove a profile", Runnable: true, Flags: append(common, agent.Flag{Name: "yes", Type: "bool", Default: "false", Usage: "skip confirmation"}), Subcommands: []agent.CommandSummary{}},
		{Name: "login", Path: "daiku auth login", Use: "daiku auth login", Runnable: true, RequiresInput: true, Subcommands: []agent.CommandSummary{}},
		{Name: "bash", Path: "daiku completion bash", Use: "daiku completion bash", Runnable: true, Subcommands: []agent.CommandSummary{}},
		{Name: "transactions", Path: "daiku transactions", Runnable: false, Subcommands: []agent.CommandSummary{{Name: "list"}}},
	}
}

func (f *fakeExecutor) Execute(ctx context.Context, args []string) Execution {
	f.mu.Lock()
	f.args = append(f.args, append([]string(nil), args...))
	f.mu.Unlock()
	if f.block {
		<-ctx.Done()
		return Execution{ExitCode: 1, Stderr: []byte(`{"ok":false,"error":{"code":"internal_error","message":"hidden"}}`)}
	}
	if f.expired {
		return Execution{ExitCode: 3, Stderr: []byte(`{"ok":false,"error":{"code":"authentication_required","message":"authentication is required","details":{"access_token":"secret"}}}`)}
	}
	return Execution{Stdout: []byte(`{"ok":true,"data":{"items":[],"refresh_token":"secret"}}`)}
}

func connect(t *testing.T, executor Executor, allowWrites bool) (*mcp.ClientSession, <-chan error) {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := New(executor, Options{AllowWrites: allowWrites, Version: "test"})
	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background(), serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, done
}

func TestDiscoveryCreatesTypedToolsPerRunnableCommand(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{}, false)
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 3 {
		t.Fatalf("tools=%d", len(result.Tools))
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	read := tools["transactions_list"]
	write := tools["transactions_create"]
	if read == nil || write == nil || !read.Annotations.ReadOnlyHint || write.Annotations.ReadOnlyHint {
		t.Fatalf("tools=%+v", tools)
	}
	schema := read.InputSchema.(map[string]any)
	properties := schema["properties"].(map[string]any)
	if properties["household"] == nil || properties["agent"] != nil {
		t.Fatalf("schema=%#v", schema)
	}
	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "household" {
		t.Fatalf("required=%#v", required)
	}
}

func TestReadToolConformanceAndSecretRedaction(t *testing.T) {
	executor := &fakeExecutor{}
	session, _ := connect(t, executor, false)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_list", Arguments: map[string]any{"household": "h1"}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	if strings.Contains(string(encoded), "secret") || !strings.Contains(string(encoded), `"items"`) {
		t.Fatalf("structured=%s", encoded)
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if got := strings.Join(executor.args[0], " "); got != "transactions list --household=h1 --agent --" {
		t.Fatalf("args=%q", got)
	}
}

func TestUnknownActionIsProtocolError(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{}, false)
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_fly"}); err == nil {
		t.Fatal("unknown tool succeeded")
	}
}

func TestWriteRejectedInReadOnlyMode(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{}, false)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_create", Arguments: map[string]any{"household": "h1", "confirm": true}})
	assertToolError(t, result, err, "write_disabled")
}

func TestWriteRequiresExplicitConfirmation(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{}, true)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_create", Arguments: map[string]any{"household": "h1"}})
	assertToolError(t, result, err, "confirmation_required")
}

func TestWriteRunsWithDoubleOptIn(t *testing.T) {
	executor := &fakeExecutor{}
	session, _ := connect(t, executor, true)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_create", Arguments: map[string]any{"household": "h1", "confirm": true}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if got := strings.Join(executor.args[0], " "); got != "transactions create --household=h1 --agent --" {
		t.Fatalf("args=%q", got)
	}
}

func TestPositionalDoubleDashValueCannotBecomeFlag(t *testing.T) {
	executor := &fakeExecutor{}
	session, _ := connect(t, executor, true)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "profile_remove", Arguments: map[string]any{"arguments": []string{"--yes"}, "confirm": true}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if got := strings.Join(executor.args[0], " "); got != "profile remove --agent -- --yes" {
		t.Fatalf("positional argument crossed flag boundary: %q", got)
	}
}

func TestExpiredTokenIsStructuredAndRedacted(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{expired: true}, false)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "transactions_list", Arguments: map[string]any{"household": "h1"}})
	assertToolError(t, result, err, "authentication_required")
	encoded, _ := json.Marshal(result.StructuredContent)
	if strings.Contains(string(encoded), "secret") {
		t.Fatalf("secret leaked: %s", encoded)
	}
}

func TestCancellationPropagatesToExecutor(t *testing.T) {
	session, _ := connect(t, &fakeExecutor{block: true}, false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "transactions_list", Arguments: map[string]any{"household": "h1"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestDisconnectStopsServer(t *testing.T) {
	session, done := connect(t, &fakeExecutor{}, false)
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after disconnect")
	}
}

func TestMalformedRPCFailsClosedWithoutPanicking(t *testing.T) {
	serverIn, clientOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	clientIn, serverOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), &fakeExecutor{}, Options{Version: "test"}, serverIn, serverOut)
	}()
	if _, err := io.WriteString(clientOut, "{malformed}\n"); err != nil {
		t.Fatal(err)
	}
	_ = clientOut.Close()
	data, _ := io.ReadAll(clientIn)
	select {
	case runErr := <-done:
		if runErr == nil {
			t.Fatalf("malformed RPC did not fail closed; response=%q", data)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after malformed disconnected input")
	}
}

func assertToolError(t *testing.T, result *mcp.CallToolResult, err error, code string) {
	t.Helper()
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(encoded), `"code":"`+code+`"`) {
		t.Fatalf("structured=%s", encoded)
	}
}
