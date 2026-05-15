// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build test

// Package agent contains logs agent component.
//
// Deprecated: use comp/logs/agent/def instead.
package agent

import (
	agent "github.com/DataDog/datadog-agent/comp/logs/agent/def"
)

// Mock implements mock-specific methods.
//
// Deprecated: use comp/logs/agent/def.Mock instead.
type Mock = agent.Mock
