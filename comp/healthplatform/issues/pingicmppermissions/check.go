// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build linux

package pingicmppermissions

import (
	"errors"
	"syscall"

	storedef "github.com/DataDog/datadog-agent/comp/healthplatform/store/def"
)

// Check attempts to create a raw ICMP socket to verify the agent has the required NET_RAW capability.
// Returns an IssueReport if the socket creation fails due to a permission error, nil otherwise.
func Check() ([]storedef.IssueReport, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			return []storedef.IssueReport{
				{
					IssueID: IssueID,
					Context: map[string]string{
						"error": err.Error(),
					},
				},
			}, nil
		}
		return nil, err
	}

	_ = syscall.Close(fd)
	return nil, nil
}
