// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build test

package storeimpl

import (
	"sync"
	"testing"

	healthplatformpayload "github.com/DataDog/agent-payload/v5/healthplatform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logmock "github.com/DataDog/datadog-agent/comp/core/log/mock"
	telemetrymock "github.com/DataDog/datadog-agent/comp/core/telemetry/mock"
	issuesmod "github.com/DataDog/datadog-agent/comp/healthplatform/issues"
)

// newTestStore builds a minimal healthPlatformImpl suitable for unit tests.
// No lifecycle, no persistence, no forwarder — just the in-memory issue map.
func newTestStore(t *testing.T) *healthPlatformImpl {
	t.Helper()
	tel := telemetrymock.New(t)
	log := logmock.New(t)

	return &healthPlatformImpl{
		log:             log,
		issueRegistry:   issuesmod.NewRegistry(),
		issues:          make(map[string]*healthplatformpayload.Issue),
		issuesMux:       sync.RWMutex{},
		persistedIssues: make(map[string]*PersistedIssue),
		persistence:     &noopPersistence{},
		metrics: telemetryMetrics{
			issuesCounter: tel.NewCounter("health_platform", "issues_detected", []string{"issue_type"}, ""),
		},
	}
}

func TestAcceptIssue_StoresIssue(t *testing.T) {
	store := newTestStore(t)

	issue := &healthplatformpayload.Issue{
		Id:       "kubelet-rbac-forbidden:node",
		Title:    "Agent Lacks Kubernetes RBAC Permissions",
		Severity: "high",
		Source:   "kubelet",
	}

	require.NoError(t, store.AcceptIssue(issue))

	got := store.GetIssue("kubelet-rbac-forbidden:node")
	require.NotNil(t, got)
	assert.Equal(t, "kubelet-rbac-forbidden:node", got.Id)
	assert.Equal(t, "Agent Lacks Kubernetes RBAC Permissions", got.Title)
	assert.Equal(t, "high", got.Severity)
}

func TestAcceptIssue_RejectsNil(t *testing.T) {
	err := newTestStore(t).AcceptIssue(nil)
	require.Error(t, err)
}

func TestAcceptIssue_RejectsEmptyID(t *testing.T) {
	err := newTestStore(t).AcceptIssue(&healthplatformpayload.Issue{Title: "no id"})
	require.Error(t, err)
}

func TestAcceptIssue_OverwritesPreviousIssue(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.AcceptIssue(&healthplatformpayload.Issue{Id: "my-issue", Title: "v1"}))
	require.NoError(t, store.AcceptIssue(&healthplatformpayload.Issue{Id: "my-issue", Title: "v2"}))

	got := store.GetIssue("my-issue")
	require.NotNil(t, got)
	assert.Equal(t, "v2", got.Title)
}

func TestAcceptIssue_ResolveIssue_RoundTrip(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.AcceptIssue(&healthplatformpayload.Issue{Id: "transient-issue", Severity: "low"}))
	require.NotNil(t, store.GetIssue("transient-issue"))

	store.ResolveIssue("transient-issue")
	assert.Nil(t, store.GetIssue("transient-issue"))
}

func TestAcceptIssue_MultipleIssues(t *testing.T) {
	store := newTestStore(t)

	for _, id := range []string{"issue-a", "issue-b", "issue-c"} {
		require.NoError(t, store.AcceptIssue(&healthplatformpayload.Issue{Id: id}))
	}

	count, issues := store.GetAllIssues()
	assert.Equal(t, 3, count)
	assert.Contains(t, issues, "issue-a")
	assert.Contains(t, issues, "issue-b")
	assert.Contains(t, issues, "issue-c")
}
