// Copyright (c) OpenFaaS Author(s) 2021. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package executor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

// FunctionRunner runs a function
type FunctionRunner interface {
	Run(f FunctionRequest) error
}

// FunctionRequest stores request for function execution
type FunctionRequest struct {
	Process     string
	ProcessArgs []string
	Environment []string

	InputReader   io.ReadCloser
	OutputWriter  io.Writer
	ContentLength *int64
}

// ForkFunctionRunner forks a process for each invocation
type ForkFunctionRunner struct {
	ExecTimeout   time.Duration
	LogPrefix     bool
	LogBufferSize int
}

// Run run a fork for each invocation
func (f *ForkFunctionRunner) Run(req FunctionRequest) error {
	log.Printf("Running: %s", req.Process)
	start := time.Now()

	var cmd *exec.Cmd
	var ctx context.Context
	if f.ExecTimeout > time.Millisecond*0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), f.ExecTimeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cmd = exec.CommandContext(ctx, req.Process, req.ProcessArgs...)

	if req.InputReader != nil {
		defer req.InputReader.Close()
		cmd.Stdin = req.InputReader
	}

	cmd.Env = req.Environment
	cmd.Stdout = req.OutputWriter

	errPipe, _ := cmd.StderrPipe()

	// Prints stderr to console and is picked up by container logging driver.
	bindLoggingPipe("stderr", errPipe, os.Stderr, f.LogPrefix, f.LogBufferSize)

	if err := cmd.Start(); err != nil {
		return err
	}

	err := cmd.Wait()
	done := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s exited: after %.2fs, error: %s", req.Process, done.Seconds(), err)
	}

	log.Printf("%s done: %.2fs secs", req.Process, done.Seconds())

	return nil
}
