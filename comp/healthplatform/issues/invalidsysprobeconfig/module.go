// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build !jetson

package invalidsysprobeconfig

import (
	"github.com/DataDog/datadog-agent/comp/healthplatform/issues"
)

// IssueID is the stable Agent Health identifier for system-probe configuration-schema violations
const IssueID = "invalid-system-probe-config"

func init() {
	issues.RegisterModuleFactoryWithDeps(NewModule)
}

type invalidSysprobeConfigModule struct {
	checker *checker
}

// NewModule constructs the module
func NewModule(deps issues.ModuleDeps) issues.Module {
	return &invalidSysprobeConfigModule{checker: newChecker(deps.SysProbeConfig)}
}

func (m *invalidSysprobeConfigModule) IssueType() string {
	return IssueID
}

func (m *invalidSysprobeConfigModule) IssueTemplate() issues.IssueTemplate {
	return InvalidSysprobeConfigIssue{}
}

// BuiltInPeriodicHealthCheck returns nil as schema validation runs only at startup
func (m *invalidSysprobeConfigModule) BuiltInPeriodicHealthCheck() *issues.BuiltInPeriodicHealthCheck {
	return nil
}

// BuiltInStartupHealthCheck runs the system-probe schema validation once at agent startup.
func (m *invalidSysprobeConfigModule) BuiltInStartupHealthCheck() *issues.BuiltInStartupHealthCheck {
	return &issues.BuiltInStartupHealthCheck{
		Source: "system-probe",
		Fn:     m.checker.Run,
	}
}
