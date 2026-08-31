package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Execution struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

type Executor interface {
	Commands() []agent.Command
	Execute(context.Context, []string) Execution
}

type Options struct {
	AllowWrites bool
	Version     string
	Logger      *slog.Logger
}

func New(executor Executor, options Options) *mcp.Server {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "daiku", Version: options.Version}, &mcp.ServerOptions{Logger: logger})
	for _, command := range executor.Commands() {
		if !command.Runnable || len(command.Subcommands) != 0 || command.RequiresInput || command.Path == "daiku mcp" || strings.HasPrefix(command.Path, "daiku completion ") {
			continue
		}
		command := command
		server.AddTool(tool(command), handler(executor, command, options.AllowWrites))
	}
	return server
}

func Run(ctx context.Context, executor Executor, options Options, in io.ReadCloser, out io.WriteCloser) error {
	return New(executor, options).Run(ctx, &mcp.IOTransport{Reader: in, Writer: out})
}

func tool(command agent.Command) *mcp.Tool {
	arguments := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Positional command arguments in order"}
	minimum, maximum := positionalBounds(command)
	arguments["minItems"] = minimum
	arguments["maxItems"] = maximum
	properties := map[string]any{"arguments": arguments}
	required := make([]string, 0)
	if minimum > 0 {
		required = append(required, "arguments")
	}
	for _, flag := range command.Flags {
		if internalFlag(flag.Name) {
			continue
		}
		name := propertyName(flag.Name)
		properties[name] = flagSchema(flag)
		if flag.Required {
			required = append(required, name)
		}
	}
	if !command.ReadOnly {
		properties["confirm"] = map[string]any{"type": "boolean", "description": "Explicitly confirm this write operation"}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) != 0 {
		schema["required"] = required
	}
	destructive := !command.ReadOnly
	return &mcp.Tool{
		Name:        toolName(command.Path),
		Description: command.Short,
		InputSchema: schema,
		Annotations: &mcp.ToolAnnotations{Title: command.Path, ReadOnlyHint: command.ReadOnly, DestructiveHint: &destructive},
	}
}

func handler(executor Executor, command agent.Command, allowWrites bool) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input map[string]json.RawMessage
		if len(request.Params.Arguments) != 0 {
			decoder := json.NewDecoder(bytes.NewReader(request.Params.Arguments))
			if err := decoder.Decode(&input); err != nil {
				return toolError("invalid_arguments", "tool arguments must be a JSON object"), nil
			}
		}
		if input == nil {
			input = map[string]json.RawMessage{}
		}
		if !command.ReadOnly {
			if !allowWrites {
				return toolError("write_disabled", "write tools require starting the MCP server with --allow-write"), nil
			}
			var confirmed bool
			if raw, ok := input["confirm"]; !ok || json.Unmarshal(raw, &confirmed) != nil || !confirmed {
				return toolError("confirmation_required", "write tools require confirm=true"), nil
			}
			delete(input, "confirm")
		}
		args, err := commandArgs(command, input)
		if err != nil {
			return toolError("invalid_arguments", err.Error()), nil
		}
		execution := executor.Execute(ctx, args)
		if ctx.Err() != nil {
			return toolError("cancelled", "operation cancelled"), nil
		}
		payload := execution.Stdout
		isError := execution.ExitCode != 0
		if isError {
			payload = execution.Stderr
		}
		structured, err := decodeEnvelope(payload)
		if err != nil {
			return toolError("invalid_command_output", "command returned an invalid structured result"), nil
		}
		structured = redact(structured).(map[string]any)
		encoded, _ := json.Marshal(structured)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: structured, IsError: isError}, nil
	}
}

func commandArgs(command agent.Command, input map[string]json.RawMessage) ([]string, error) {
	args := strings.Fields(strings.TrimPrefix(command.Path, "daiku "))
	var positional []string
	if raw, ok := input["arguments"]; ok {
		if err := json.Unmarshal(raw, &positional); err != nil {
			return nil, errors.New("arguments must be an array of strings")
		}
		delete(input, "arguments")
	}
	flags := map[string]agent.Flag{}
	for _, flag := range command.Flags {
		if !internalFlag(flag.Name) {
			flags[propertyName(flag.Name)] = flag
		}
	}
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := input[name]
		flag, ok := flags[name]
		if !ok {
			return nil, fmt.Errorf("unknown argument %q", name)
		}
		value, err := flagValue(flag.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("argument %q: %w", name, err)
		}
		args = append(args, "--"+flag.Name+"="+value)
	}
	args = append(args, "--agent", "--")
	args = append(args, positional...)
	return args, nil
}

func flagValue(kind string, raw json.RawMessage) (string, error) {
	switch kind {
	case "bool":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("must be a boolean")
		}
		return strconv.FormatBool(value), nil
	case "int", "int64", "uint", "uint64", "float64":
		var value json.Number
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return "", errors.New("must be a number")
		}
		return value.String(), nil
	case "stringSlice", "stringArray":
		var value []string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("must be an array of strings")
		}
		return strings.Join(value, ","), nil
	default:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("must be a string")
		}
		return value, nil
	}
}

func flagSchema(flag agent.Flag) map[string]any {
	typeName := "string"
	if flag.Type == "bool" {
		typeName = "boolean"
	}
	if flag.Type == "int" || flag.Type == "int64" || flag.Type == "uint" || flag.Type == "uint64" || flag.Type == "float64" {
		typeName = "number"
	}
	if flag.Type == "stringSlice" || flag.Type == "stringArray" {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": flag.Usage}
	}
	return map[string]any{"type": typeName, "description": flag.Usage}
}

func toolName(path string) string {
	return strings.NewReplacer("daiku ", "", "-", "_", " ", "_").Replace(path)
}

func positionalBounds(command agent.Command) (int, int) {
	use := strings.Fields(command.Use)
	path := strings.Fields(command.Path)
	if len(use) <= len(path) {
		return 0, 0
	}
	minimum, maximum := 0, 0
	for _, token := range use[len(path):] {
		if strings.HasPrefix(token, "[") {
			maximum++
			continue
		}
		minimum++
		maximum++
	}
	return minimum, maximum
}
func propertyName(name string) string { return strings.ReplaceAll(name, "-", "_") }
func internalFlag(name string) bool {
	return name == "agent" || name == "json" || name == "no-input" || name == "language" || name == "help"
}

func decodeEnvelope(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if _, ok := value["ok"].(bool); !ok {
		return nil, errors.New("missing ok")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return value, nil
}

func toolError(code, message string) *mcp.CallToolResult {
	value := map[string]any{"ok": false, "error": map[string]any{"code": code, "message": message}}
	encoded, _ := json.Marshal(value)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: value, IsError: true}
}

func redact(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || lower == "authorization" {
				continue
			}
			out[key] = redact(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = redact(child)
		}
		return out
	default:
		return value
	}
}
