// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// readAllAvailable reads from reader until it receives an EOF or the timeout
// expires; the expectation is that timeout should be very short. Essentially,
// this is a "nonblocking" read, with nonblocking in quotes because it does
// block for `timeout`. This exists because in testing we found some
// circumstances in which particular devices would print an error message but
// not send an EOF.
func readAllAvailable(ctx context.Context, reader io.Reader, timeout time.Duration) (string, error) {
	ch := make(chan result, 1)
	go func() {
		for {
			buffer := make([]byte, 1000)
			n, err := reader.Read(buffer)
			if err == nil && n < len(buffer) {
				// if we didn't fill the buffer then we assume the output is
				// done, even if the source didn't send an EOF.
				err = io.EOF
			}
			ch <- result{string(buffer[:n]), err}
			if err != nil || n < len(buffer) {
				return
			}
		}
	}()
	var output strings.Builder
	for {
		select {
		case r := <-ch:
			output.WriteString(r.message)
			if r.err != nil {
				if errors.Is(r.err, io.EOF) {
					return output.String(), nil
				}
				return output.String(), r.err
			}
		case <-time.After(timeout):
			return output.String(), nil
		case <-ctx.Done():
			return output.String(), ctx.Err()
		}
	}

}

// readSCPResponse reads an SCP response from the reader. If the first byte is
// 0, the command succeeded and nothing more is read. Otherwise, the remaining
// bytes are read and returned as the error message.
func readSCPResponse(ctx context.Context, reader io.Reader) error {
	buffer := make([]byte, 1)
	if _, err := reader.Read(buffer); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("expected response but got EOF")
		}
		return err
	}
	if buffer[0] > 0 {
		message, err := readAllAvailable(ctx, reader, time.Millisecond*10)
		if err != nil {
			return err
		}
		return errors.New(string(message))
	}
	return nil
}

func executeSCP(ctx context.Context, session *ssh.Session, cmdStr string, filename string, data string) (string, error) {
	// Get I/O handles
	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", err
	}
	// Start the SCP command
	if err := session.Start(cmdStr); err != nil {
		return "", err
	}
	// Write the scp control line
	if _, err := fmt.Fprintln(stdin, "C0600", len(data), filename); err != nil {
		return "", fmt.Errorf("writing control line: %w", err)
	}
	if err := readSCPResponse(ctx, stdout); err != nil {
		return "", err
	}
	// Write the data
	if _, err = fmt.Fprint(stdin, data+"\x00"); err != nil {
		return "", fmt.Errorf("writing data: %w", err)
	}
	if err := readSCPResponse(ctx, stdout); err != nil {
		return "", err
	}
	stdin.Close()
	if err := session.Wait(); err != nil {
		return "", err
	}
	// Read any additional output that the command provides
	result, err := io.ReadAll(stdout)
	return string(result), err
}
