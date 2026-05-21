// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build linux

// Package pingicmppermissions provides a complete issue module for ping ICMP socket permission problems.
// It includes both detection (built-in health check) and remediation (issue template with fix steps).
package pingicmppermissions

import (
	"github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/healthplatform/issues"
	storedef "github.com/DataDog/datadog-agent/comp/healthplatform/store/def"
)

func init() {
	issues.RegisterModuleFactory(NewModule)
}

const (
	// IssueType is the template type identifier for ping ICMP permission issues
	IssueType = "ping-icmp-permissions"

	// IssueID is the unique instance id used when reporting this issue
	IssueID = "ping-icmp-permissions"
)

// pingICMPPermissionsModule implements issues.Module
type pingICMPPermissionsModule struct {
	template *PingICMPPermissionsIssue
}

// NewModule creates a new ping ICMP permissions issue module
func NewModule(_ config.Component) issues.Module {
	return &pingICMPPermissionsModule{
		template: NewPingICMPPermissionsIssue(),
	}
}

// IssueType returns the template type identifier for this issue type
func (m *pingICMPPermissionsModule) IssueType() string {
	return IssueType
}

// IssueTemplate returns the template for building complete issues
func (m *pingICMPPermissionsModule) IssueTemplate() issues.IssueTemplate {
	return m.template
}

// BuiltInPeriodicHealthCheck returns nil — ICMP permission checks run once at startup, not periodically.
func (m *pingICMPPermissionsModule) BuiltInPeriodicHealthCheck() *issues.BuiltInPeriodicHealthCheck {
	return nil
}

// BuiltInStartupHealthCheck runs the ICMP socket permission check once at agent startup.
func (m *pingICMPPermissionsModule) BuiltInStartupHealthCheck() *issues.BuiltInStartupHealthCheck {
	return &issues.BuiltInStartupHealthCheck{
		Source: "ping",
		Fn: func() ([]storedef.IssueReport, error) {
			return Check()
		},
	}
}
