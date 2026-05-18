// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build windows

package windowsagentinstability

import (
	"fmt"
	"strconv"
	"time"

	"github.com/DataDog/agent-payload/v5/healthplatform"
	evtapi "github.com/DataDog/datadog-agent/pkg/util/winutil/eventlog/api"
	winevtapi "github.com/DataDog/datadog-agent/pkg/util/winutil/eventlog/api/windows"
)

const (
	// crashThreshold is the number of SCM crash events in the time window that triggers an issue
	crashThreshold = 2

	// timeWindow is the duration to look back for crash events
	timeWindow = 24 * time.Hour

	// systemLog is the Windows System event log channel
	systemLog = "System"

	// scmProvider is the Windows Service Control Manager event source name
	scmProvider = "Service Control Manager"

	// datadogServiceName is the display name of the Datadog Agent Windows service as
	// recorded in SCM events. Event IDs 7031 and 7034 include this name in param1.
	datadogServiceName = "Datadog Agent"
)

// Check queries the Windows System Event Log for recent Datadog Agent service termination
// events written by the Service Control Manager (SCM). SCM event IDs 7034 and 7031 are
// written unconditionally by Windows when a service exits unexpectedly, regardless of
// whether the agent itself had time to write anything to its own log file.
//
// If more than crashThreshold events are found in the last timeWindow, it returns an
// IssueReport. If the event log is inaccessible the function returns nil to avoid
// false positives.
func Check() (*healthplatform.IssueReport, error) {
	count, err := countSCMCrashEvents(timeWindow)
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	if count <= crashThreshold {
		return nil, nil
	}

	return &healthplatform.IssueReport{
		IssueId: IssueID,
		Context: map[string]string{
			"crashCount": strconv.Itoa(count),
			"timeWindow": "24h",
		},
		Tags: []string{"windows", "service-crash", "stability"},
	}, nil
}

// countSCMCrashEvents returns the number of SCM events (7031 or 7034) for the Datadog
// Agent service that occurred within the given time window.
func countSCMCrashEvents(window time.Duration) (int, error) {
	api := winevtapi.New()
	query := buildXPathQuery(window)

	// EvtQueryReverseDirection reads newest-first; we just count so direction does not matter.
	resultSet, err := api.EvtQuery(
		evtapi.EventSessionHandle(0),
		systemLog,
		query,
		evtapi.EvtQueryChannelPath|evtapi.EvtQueryReverseDirection,
	)
	if err != nil {
		return 0, fmt.Errorf("EvtQuery on System log failed: %w", err)
	}
	defer evtapi.EvtCloseResultSet(api, resultSet)

	count := 0
	batch := make([]evtapi.EventRecordHandle, 16)
	for {
		records, err := api.EvtNext(resultSet, batch, uint(len(batch)), 0)
		if err != nil || len(records) == 0 {
			// ERROR_NO_MORE_ITEMS is the normal end-of-query signal; any error here means done.
			break
		}
		count += len(records)
		for _, r := range records {
			evtapi.EvtCloseRecord(api, r)
		}
	}

	return count, nil
}

// buildXPathQuery returns an XPath 1.0 query that selects SCM service-crash events for
// the Datadog Agent service within the given time window.
//
// Event IDs used:
//   - 7034: "The <service> service terminated unexpectedly."
//   - 7031: "The <service> service terminated unexpectedly. It has done this N time(s)."
//
// timediff(@SystemTime) is a Windows-specific XPath extension that returns the elapsed
// milliseconds since the event timestamp, allowing server-side time filtering.
func buildXPathQuery(window time.Duration) string {
	ms := int64(window / time.Millisecond)
	return fmt.Sprintf(
		"*[System[Provider[@Name='%s'] and (EventID=7031 or EventID=7034) and TimeCreated[timediff(@SystemTime) <= %d]] and EventData[Data='%s']]",
		scmProvider,
		ms,
		datadogServiceName,
	)
}
