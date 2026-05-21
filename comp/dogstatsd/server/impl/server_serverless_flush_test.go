// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build test

package serverimpl

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/comp/dogstatsd/listeners"
	"github.com/DataDog/datadog-agent/pkg/metrics"
)

// TestServerlessFlushReturnsWhenNotStarted is a regression guard for the
// deadlock fix: when the server was never started there are no workers
// consuming the flush channel, so ServerlessFlush must return promptly
// rather than blocking on an unbuffered send.
func TestServerlessFlushReturnsWhenNotStarted(t *testing.T) {
	cfg := make(map[string]interface{})
	cfg["dogstatsd_port"] = listeners.RandomPortName

	// fulfillDepsWithInactiveServerlessServer constructs a *dsdServer with
	// ServerlessMode=true that has not been started, so s.workers is empty
	// and IsRunning() is false. This mirrors the production path where
	// serverless-init runs without an api_key and the DogStatsD server is
	// never started — exercises the exact code path used in production.
	_, s := fulfillDepsWithInactiveServerlessServer(t, cfg)
	requireStopped(t, s)

	done := make(chan struct{})
	go func() {
		s.ServerlessFlush(0)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("ServerlessFlush blocked when the server was not started — deadlock regression")
	}
}

// TestServerlessFlushFansOutToAllWorkers verifies the per-worker fan-out:
// every worker's batcher must run flush() exactly once so that samples
// queued on every worker reach the time sampler. Regression guard for the
// "shared channel let one fast worker drain N sends while a busy peer never
// received" bug raised in the audit.
func TestServerlessFlushFansOutToAllWorkers(t *testing.T) {
	const workerCount = 2

	cfg := make(map[string]interface{})
	cfg["dogstatsd_port"] = listeners.RandomPortName
	cfg["dogstatsd_workers_count"] = workerCount

	// Use the serverless-mode helper so workers run newServerlessBatcher —
	// the exact production code path exercised by serverless-init. A
	// regression in newServerlessBatcher.flush would otherwise pass the
	// default (non-serverless) batcher test.
	deps := fulfillDepsWithServerlessConfigOverride(t, cfg)
	s := deps.Server.(*dsdServer)
	requireStart(t, s)
	require.Len(t, s.workers, workerCount, "expected the configured number of workers")

	// Workers are parked on select with no packetsIn traffic, so direct
	// batcher writes from this test goroutine are safe. The unbuffered
	// flushChan send/recv inside ServerlessFlush provides happens-before
	// with the subsequent batcher.flush() that runs on the worker goroutine.
	//
	// Each worker gets a uniquely-named sample so the assertion can verify
	// EVERY worker drained, not just total count — a regression where one
	// fast worker double-fires while a peer never flushes would still match
	// the total count but would miss one of the unique names.
	expectedNames := make(map[string]struct{}, workerCount)
	for i, w := range s.workers {
		name := "serverless.flush.test.worker." + strconv.Itoa(i)
		expectedNames[name] = struct{}{}
		w.batcher.appendSample(metrics.MetricSample{
			Name:       name,
			Value:      float64(i + 1),
			Mtype:      metrics.GaugeType,
			SampleRate: 1,
		})
	}

	s.ServerlessFlush(0)

	// Wait for at least workerCount samples, then verify each unique
	// per-worker name appears at least once — proves every worker's
	// batcher.flush() actually ran.
	samples, _ := deps.Demultiplexer.WaitForNumberOfSamples(workerCount, 0, 2*time.Second)
	gotNames := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		gotNames[sample.Name] = struct{}{}
	}
	for name := range expectedNames {
		assert.Contains(t, gotNames, name, "expected sample from each worker — fan-out regression: missing %s", name)
	}
}
