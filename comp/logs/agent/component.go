// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

// Package agent contains logs agent component.
//
// Deprecated: use comp/logs/agent/def instead.
package agent

import (
	agent "github.com/DataDog/datadog-agent/comp/logs/agent/def"
)

// Component is the component type.
//
// Deprecated: use comp/logs/agent/def.Component instead.
type Component = agent.Component

// ServerlessLogsAgent is a compat version of the component for the serverless agent.
//
// Deprecated: use comp/logs/agent/def.ServerlessLogsAgent instead.
type ServerlessLogsAgent = agent.ServerlessLogsAgent
