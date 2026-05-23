// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package remote

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"golang.org/x/crypto/ssh"
)

// Validator contains rules for validating the output of a command - requiring
// specific regexes to be present or absent in stdout and/or stderr.
type Validator struct {
	Require []*regexp.Regexp `mapstructure:"stdout"`
	Reject  []*regexp.Regexp `mapstructure:"stdout"`
}

func (v *Validator) Validate(text string) error {
	for _, rule := range v.Require {
		if !rule.MatchString(text) {
			return fmt.Errorf("does not match required regex %q", rule)
		}
	}
	for _, rule := range v.Reject {
		if rule.MatchString(text) {
			return fmt.Errorf("matches failure regex %q", rule)
		}
	}
	return nil
}

// Command represents a single command plus zero or more regexes to run against
// the combined stdout/stderr of that command.
type Command struct {
	Command   string    `mapstructure:"command"`
	Validator Validator `mapstructure:"validator"`
}

// SCPCommand represents a command that expects to receive valid scp input via
// stdin. The actual command run over SSH will be `<RemoteCommand> -qt <FilePath>`
type SCPCommand struct {
	RemoteCommand string `mapstructure:"remote_command"`
	Filepath      string `mapstructure:"filepath"`
	// usually this should be empty - scp does not print output on most systems.
	Validator Validator `mapstructure:"validator"`
}

type result struct {
	message string
	err     error
}

// Execute runs a command and validates the output with its validation rules.
// The validation runs on the combined stdout and stderr of the command.
func ExecuteCommand(ctx context.Context, session *ssh.Session, cmd *Command) (string, error) {
	ch := make(chan result, 1)
	go func() {
		output, err := session.CombinedOutput(cmd.Command)
		ch <- result{string(output), err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return "", r.err
		}
		return r.message, cmd.Validator.Validate(r.message)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ExecuteSCP executes an SCP command, sending the given data over SSH.
func ExecuteSCP(ctx context.Context, session *ssh.Session, cmd *SCPCommand, data string) (string, error) {
	cmdStr := fmt.Sprintf("%s -t %s", cmd.RemoteCommand, cmd.Filepath)
	result, err := executeSCP(ctx, session, cmdStr, filepath.Base(cmd.Filepath), data)
	if err != nil {
		return "", fmt.Errorf("scp command %q failed: %w", cmdStr, err)
	}
	if err := cmd.Validator.Validate(result); err != nil {
		return result, fmt.Errorf("scp command %q bad output: %w", cmdStr, err)
	}
	return result, nil
}
