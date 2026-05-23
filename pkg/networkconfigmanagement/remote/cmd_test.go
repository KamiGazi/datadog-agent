// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build test

package remote

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func mustSession(t *testing.T, client *ssh.Client) *ssh.Session {
	session, err := client.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() {
		session.Close()
	})
	return session
}

func TestCommand(t *testing.T) {
	srv := StartFakeSSHServer(t, map[string]FakeResponse{
		"show version": Ok("Fakesco fOS\n"),
		"show venison": Fail("bad command", 1),
	})
	client := MustConnect(t, srv)

	for _, tc := range []struct {
		name      string
		cmd       *Command
		expected  string
		expectErr bool
	}{{
		name: "unchecked_command",
		cmd: &Command{
			Command: "show version",
		},
		expected: "Fakesco fOS\n",
	}, {
		name: "valid_command",
		cmd: &Command{
			Command: "show version",
			Validator: Validator{
				Require: []*regexp.Regexp{regexp.MustCompile("Fakesco")},
				Reject:  []*regexp.Regexp{regexp.MustCompile("Realco")},
			},
		},
		expected: "Fakesco fOS\n",
	}, {
		name: "missing_req",
		cmd: &Command{
			Command: "show version",
			Validator: Validator{
				Require: []*regexp.Regexp{regexp.MustCompile("Realco")},
			},
		},
		expectErr: true,
	}, {
		name: "has_rejection",
		cmd: &Command{
			Command: "show version",
			Validator: Validator{
				Reject: []*regexp.Regexp{regexp.MustCompile("Fakesco")},
			},
		},
		expectErr: true,
	}, {
		name: "command_fails",
		cmd: &Command{
			Command: "show venison",
			Validator: Validator{
				Require: []*regexp.Regexp{regexp.MustCompile("Fakesco")},
				Reject:  []*regexp.Regexp{regexp.MustCompile("Realco")},
			},
		},
		expectErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			session := mustSession(t, client)
			result, err := ExecuteCommand(context.Background(), session, tc.cmd)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, string(result))
			}
		})
	}
}

func TestSCPCommand(t *testing.T) {
	srv := StartFakeSSHServerWithFunc(t, func(command string, stdin io.Reader, stdout, stderr io.Writer) (returnCode uint32) {
		switch command {
		case "scp -t /tmp/foo.txt":
			stdout.Write([]byte{0, 0})
			// wait for the other end to write some data so we don't close stdin early
			stdin.Read(make([]byte, 100))
			return 0
		case "scp -t /tmp/permission.txt":
			fmt.Fprint(stdout, "\x01permission denied")
			stdin.Read(make([]byte, 100))
			return 1
		case "scp -t /tmp/hang.txt":
			fmt.Fprint(stdout, "\x01sending feedback but never closing the stream")
			for { // do nothing, forever.
				time.Sleep(time.Second)
			}
		case "scp -t /tmp/feedback.txt":
			fmt.Fprint(stdout, "\x00\x00returning feedback")
			// wait for the other end to write some data so we don't close stdin early
			stdin.Read(make([]byte, 100))
			return 0
		}
		fmt.Fprintf(stderr, "unknown command: %s\n", command)
		return 127
	})
	client := MustConnect(t, srv)
	for _, tc := range []struct {
		name      string
		cmd       *SCPCommand
		expected  string
		expectErr string
	}{{
		name: "unchecked_command",
		cmd: &SCPCommand{
			RemoteCommand: "scp",
			Filepath:      "/tmp/foo.txt",
		},
		expected: "",
	}, {
		name: "command_that_hangs",
		cmd: &SCPCommand{
			RemoteCommand: "scp",
			Filepath:      "/tmp/hang.txt",
		},
		expectErr: "sending feedback but never closing the stream",
	}, {
		name: "failing_command",
		cmd: &SCPCommand{
			RemoteCommand: "scp",
			Filepath:      "/tmp/permission.txt",
		},
		expectErr: "permission denied",
	}, {
		name: "validate_response",
		cmd: &SCPCommand{
			RemoteCommand: "scp",
			Filepath:      "/tmp/feedback.txt",
			Validator: Validator{
				Require: []*regexp.Regexp{
					regexp.MustCompile("feedback"),
				},
			},
		},
		expected: "returning feedback",
	}, {
		name: "invalid_response",
		cmd: &SCPCommand{
			RemoteCommand: "scp",
			Filepath:      "/tmp/feedback.txt",
			Validator: Validator{
				Reject: []*regexp.Regexp{
					regexp.MustCompile("feedback"),
				},
			},
		},
		expectErr: "feedback",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			session := mustSession(t, client)
			response, err := ExecuteSCP(context.Background(), session, tc.cmd, "this is the data")
			if tc.expectErr != "" {
				assert.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, response)
			}
		})
	}

}
