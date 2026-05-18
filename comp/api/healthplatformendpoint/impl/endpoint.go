// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

// Package impl provides the health platform IPC endpoint implementation
package impl

import (
	"encoding/json"
	"net/http"

	api "github.com/DataDog/datadog-agent/comp/api/api/def"
	storedef "github.com/DataDog/datadog-agent/comp/healthplatform/store/def"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/DataDog/datadog-agent/pkg/util/option"
)

// Requires holds the components needed by the health platform endpoint provider.
type Requires struct {
	HealthPlatform option.Option[storedef.Component]
}

// Provider exposes the health platform IPC endpoints to the agent CMD server.
type Provider struct {
	ReportIssueEndpoint  api.AgentEndpointProvider
	ResolveIssueEndpoint api.AgentEndpointProvider
}

// NewHealthPlatformEndpointProvider returns a Provider wired with the report and resolve handlers.
func NewHealthPlatformEndpointProvider(requires Requires) Provider {
	return Provider{
		ReportIssueEndpoint:  api.NewAgentEndpointProvider(reportIssueHandler(requires.HealthPlatform), "/health-platform/issue", "POST"),
		ResolveIssueEndpoint: api.NewAgentEndpointProvider(resolveIssueHandler(requires.HealthPlatform), "/health-platform/resolve", "POST"),
	}
}

// reportIssueHandler returns an HTTP handler that decodes an IssueReport from the request body
// and forwards it to the health platform store.
func reportIssueHandler(hp option.Option[storedef.Component]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := hp.Get()
		if !ok {
			http.Error(w, "health platform store not available", http.StatusServiceUnavailable)
			return
		}

		var report storedef.IssueReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := store.ReportIssue(report); err != nil {
			log.Warnf("health platform: failed to report issue %q: %v", report.IssueID, err)
			http.Error(w, "failed to report issue: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// resolveIssueHandler returns an HTTP handler that decodes an issue ID from the request body
// and resolves the corresponding issue in the health platform store.
func resolveIssueHandler(hp option.Option[storedef.Component]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := hp.Get()
		if !ok {
			http.Error(w, "health platform store not available", http.StatusServiceUnavailable)
			return
		}

		var body struct {
			IssueID string `json:"issue_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		store.ResolveIssue(body.IssueID)
		w.WriteHeader(http.StatusOK)
	}
}
