// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package remote

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/crypto/ssh"

	"github.com/DataDog/datadog-agent/pkg/networkconfigmanagement/profile"
)

type result struct {
	message string
	err     error
}

// Execute runs a command and validates the output with its validation rules.
// The validation runs on the combined stdout and stderr of the command.
func ExecuteCommand(ctx context.Context, session *ssh.Session, cmd *profile.Command) (string, error) {
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
func ExecuteSCP(ctx context.Context, session *ssh.Session, cmd *profile.SCPCommand, data string) (string, error) {
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
