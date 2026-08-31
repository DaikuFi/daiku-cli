package main

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/spf13/cobra"
)

type moduleFunc func(*cobra.Command)

func (f moduleFunc) Register(root *cobra.Command) { f(root) }

func TestCommandExecutorSerializesConcurrentExecutions(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var counters sync.Mutex
	inFlight := 0
	maxInFlight := 0

	executor := &commandExecutor{
		newApp: func(ctx context.Context, in io.Reader, out, errOut io.Writer) *cli.App {
			return cli.New(
				cli.WithContext(ctx),
				cli.WithIO(in, out, errOut),
				cli.WithModule(moduleFunc(func(root *cobra.Command) {
					root.AddCommand(&cobra.Command{Use: "block", RunE: func(*cobra.Command, []string) error {
						counters.Lock()
						inFlight++
						if inFlight > maxInFlight {
							maxInFlight = inFlight
						}
						counters.Unlock()
						entered <- struct{}{}
						<-release
						counters.Lock()
						inFlight--
						counters.Unlock()
						return nil
					}})
				})),
			)
		},
		gate: make(chan struct{}, 1),
	}

	done := make(chan struct{}, 2)
	for range 2 {
		go func() {
			executor.Execute(context.Background(), []string{"block"})
			done <- struct{}{}
		}()
	}

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first execution did not start")
	}
	select {
	case <-entered:
		t.Fatal("second execution entered before the first completed")
	case <-time.After(50 * time.Millisecond):
	}

	release <- struct{}{}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("second execution did not start after the first completed")
	}
	release <- struct{}{}
	for range 2 {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("execution did not finish")
		}
	}

	counters.Lock()
	defer counters.Unlock()
	if maxInFlight != 1 {
		t.Fatalf("max concurrent executions = %d, want 1", maxInFlight)
	}
}

func TestCommandExecutorCancelledWhileQueuedNeverExecutes(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	executor := &commandExecutor{
		newApp: func(ctx context.Context, in io.Reader, out, errOut io.Writer) *cli.App {
			return cli.New(
				cli.WithContext(ctx),
				cli.WithIO(in, out, errOut),
				cli.WithModule(moduleFunc(func(root *cobra.Command) {
					root.AddCommand(&cobra.Command{Use: "write", RunE: func(*cobra.Command, []string) error {
						entered <- struct{}{}
						<-release
						return nil
					}})
				})),
			)
		},
		gate: make(chan struct{}, 1),
	}

	firstDone := make(chan struct{})
	go func() {
		executor.Execute(context.Background(), []string{"write"})
		close(firstDone)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first write did not start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan struct{})
	go func() {
		executor.Execute(ctx, []string{"write"})
		close(secondDone)
	}()
	cancel()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("cancelled queued write did not return promptly")
	}
	select {
	case <-entered:
		t.Fatal("cancelled queued write entered command handler")
	default:
	}

	release <- struct{}{}
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first write did not finish")
	}
	select {
	case <-entered:
		t.Fatal("cancelled queued write entered after the gate was released")
	default:
	}
}
