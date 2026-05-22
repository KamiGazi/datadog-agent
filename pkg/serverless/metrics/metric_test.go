// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build test

package metrics

import (
	"context"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core"
	delegatedauthmock "github.com/DataDog/datadog-agent/comp/core/delegatedauth/mock"
	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameimpl"
	secrets "github.com/DataDog/datadog-agent/comp/core/secrets/def"
	secretsmock "github.com/DataDog/datadog-agent/comp/core/secrets/mock"
	nooptagger "github.com/DataDog/datadog-agent/comp/core/tagger/impl-noop"
	"github.com/DataDog/datadog-agent/comp/dogstatsd/listeners"
	filterlistmock "github.com/DataDog/datadog-agent/comp/filterlist/fx-mock"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder/resolver"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder/transaction"
	haagentmock "github.com/DataDog/datadog-agent/comp/haagent/mock"
	logscompression "github.com/DataDog/datadog-agent/comp/serializer/logscompression/fx-mock"
	metricscompression "github.com/DataDog/datadog-agent/comp/serializer/metricscompression/fx-mock"
	"github.com/DataDog/datadog-agent/pkg/aggregator"
	configmock "github.com/DataDog/datadog-agent/pkg/config/mock"
	pkgconfigsetup "github.com/DataDog/datadog-agent/pkg/config/setup"
	configutils "github.com/DataDog/datadog-agent/pkg/config/utils"
	pkgmetrics "github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/serverless/metrics/metricstest"
	"github.com/DataDog/datadog-agent/pkg/util/cache"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
	"github.com/DataDog/datadog-agent/pkg/util/hostname"
)

func TestMain(m *testing.M) {
	// setting the hostname cache saves about 1s when starting the metric agent
	cacheKey := cache.BuildAgentKey("hostname")
	cache.Cache.Set(cacheKey, hostname.Data{}, cache.NoExpiration)
	os.Exit(m.Run())
}

func TestConstructionDoesNotBlock(t *testing.T) {
	if os.Getenv("CI") == "true" && runtime.GOOS == "darwin" {
		t.Skip("known to fail on the macOS Gitlab runners because of the already running Agent")
	}
	mockConfig := configmock.New(t)
	pkgconfigsetup.LoadDatadog(mockConfig, secretsmock.New(t), delegatedauthmock.New(t), nil)
	deps := metricstest.New(t, nooptagger.NewComponent())
	metricAgent := &ServerlessMetricAgent{Demux: deps.Demux}
	assert.NotNil(t, metricAgent.Demux)
}

func TestRaceFlushVersusParsePacket(t *testing.T) {
	mockConfig := configmock.New(t)
	pkgconfigsetup.LoadDatadog(mockConfig, secretsmock.New(t), delegatedauthmock.New(t), nil)
	mockConfig.SetDefault("dogstatsd_port", listeners.RandomPortName)

	deps := metricstest.New(t, nooptagger.NewComponent())

	url := deps.DogstatsdServer.UDPLocalAddr()
	conn, err := net.Dial("udp", url)
	require.NoError(t, err, "cannot connect to DSD socket")
	defer conn.Close()

	finish := &sync.WaitGroup{}
	finish.Add(2)

	go func(wg *sync.WaitGroup) {
		for i := 0; i < 1000; i++ {
			conn.Write([]byte("daemon:666|g|#sometag1:somevalue1,sometag2:somevalue2"))
			time.Sleep(10 * time.Nanosecond)
		}
		wg.Done()
	}(finish)

	go func(wg *sync.WaitGroup) {
		for i := 0; i < 1000; i++ {
			deps.DogstatsdServer.ServerlessFlush(time.Second * 10)
		}
		wg.Done()
	}(finish)

	finish.Wait()
}

// countingForwarder wraps NoopForwarder, provides a real domain resolver so that
// the serializer's pipeline path is exercised, and counts sketch transactions.
type countingForwarder struct {
	defaultforwarder.NoopForwarder
	sketchCount atomic.Int64
	resolvers   []resolver.DomainResolver
}

func newCountingForwarder() *countingForwarder {
	r, _ := resolver.NewSingleDomainResolver("https://fake.datadoghq.com",
		[]configutils.APIKeys{configutils.NewAPIKeys("api_key", "fakeapikey")})
	return &countingForwarder{resolvers: []resolver.DomainResolver{r}}
}

// GetDomainResolvers returns the fake resolver so buildPipelines creates a pipeline.
func (f *countingForwarder) GetDomainResolvers() []resolver.DomainResolver {
	return f.resolvers
}

// SubmitTransaction increments the sketch counter when a sketch-series transaction arrives.
func (f *countingForwarder) SubmitTransaction(txn *transaction.HTTPTransaction) error {
	if strings.Contains(txn.Endpoint.Name, "sketch") {
		f.sketchCount.Add(1)
	}
	return nil
}

// SubmitSketchSeries is kept for interface compliance but is not called by the pipeline path.
func (f *countingForwarder) SubmitSketchSeries(_ transaction.BytesPayloads, _ http.Header) error {
	return nil
}

// TestStopDrainsBeforeFlush asserts that ServerlessMetricAgent.Stop(ctx) reliably
// drains the timeSamplerWorker's samplesChan, so a sample submitted via
// AddEnhancedMetric immediately before ForceFlushToSerializer is delivered to the
// serializer. Without the Stop(ctx) synchronization, the worker's select can pick
// flushChan over samplesChan and flush before the sample is enqueued — a race that
// drops ~50% of the samples in practice. 100 iterations exercise that race.
func TestStopDrainsBeforeFlush(t *testing.T) {
	mockConfig := configmock.New(t)
	pkgconfigsetup.LoadDatadog(mockConfig, secretsmock.New(t), delegatedauthmock.New(t), nil)

	cf := newCountingForwarder()

	deps := fxutil.Test[aggregator.TestDeps](t,
		fx.Provide(func() secrets.Component { return secretsmock.New(t) }),
		fx.Provide(func() defaultforwarder.Component { return cf }),
		core.MockBundle(),
		hostnameimpl.MockModule(),
		haagentmock.Module(),
		logscompression.MockModule(),
		metricscompression.MockModule(),
		filterlistmock.MockModule(),
	)

	const iterations = 100
	for i := 0; i < iterations; i++ {
		opts := aggregator.DefaultAgentDemultiplexerOptions()
		opts.FlushInterval = time.Hour // disable automatic flushes
		opts.DontStartForwarders = true
		demux := aggregator.InitAndStartAgentDemultiplexerForTest(deps, opts, "")

		agent := New(demux, Tags{})
		agent.AddEnhancedMetric("test.metric", 1.0, pkgmetrics.MetricSourceServerless, 1000.0)

		// Stop(ctx) must drain the worker's samplesChan before returning so the
		// ForceFlushToSerializer below reliably observes the sample. Without it,
		// the worker's select can pick flushChan first and drop the sample.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		require.NoError(t, agent.Stop(ctx))
		cancel()

		demux.ForceFlushToSerializer(time.Now(), true)
		demux.Stop(false)
	}

	require.Equal(t, int64(iterations), cf.sketchCount.Load(),
		"every AddEnhancedMetric followed by Stop must produce exactly one sketch flush")
}

// wrappedDemux mirrors the demultiplexerimpl.demultiplexer wrapper struct that
// Fx actually supplies to ServerlessMetricAgent: the AggregatorDemultiplexer
// interface holds a struct that embeds *aggregator.AgentDemultiplexer rather
// than the pointer itself. A concrete *aggregator.AgentDemultiplexer type
// assertion would silently fail on this value, causing Stop to no-op in
// production — the regression this test guards against.
type wrappedDemux struct {
	*aggregator.AgentDemultiplexer
}

func TestStopDrainsThroughWrappedDemux(t *testing.T) {
	mockConfig := configmock.New(t)
	pkgconfigsetup.LoadDatadog(mockConfig, secretsmock.New(t), delegatedauthmock.New(t), nil)

	cf := newCountingForwarder()

	deps := fxutil.Test[aggregator.TestDeps](t,
		fx.Provide(func() secrets.Component { return secretsmock.New(t) }),
		fx.Provide(func() defaultforwarder.Component { return cf }),
		core.MockBundle(),
		hostnameimpl.MockModule(),
		haagentmock.Module(),
		logscompression.MockModule(),
		metricscompression.MockModule(),
		filterlistmock.MockModule(),
	)

	opts := aggregator.DefaultAgentDemultiplexerOptions()
	opts.FlushInterval = time.Hour
	opts.DontStartForwarders = true
	demux := aggregator.InitAndStartAgentDemultiplexerForTest(deps, opts, "")
	defer demux.Stop(false)

	agent := New(wrappedDemux{AgentDemultiplexer: demux}, Tags{})
	agent.AddEnhancedMetric("test.metric", 1.0, pkgmetrics.MetricSourceServerless, 1000.0)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, agent.Stop(ctx))

	demux.ForceFlushToSerializer(time.Now(), true)
	require.Equal(t, int64(1), cf.sketchCount.Load(),
		"Stop must drain pending samples even when Demux is a wrapper embedding *AgentDemultiplexer")
}

// fakeFlusher implements ServerlessFlusher and records each call, optionally
// blocking for a configurable duration so tests can exercise the flushTimeout
// bound in Shutdown.
type fakeFlusher struct {
	calls   atomic.Int64
	block   time.Duration
	lastArg atomic.Int64 // time.Duration arg, stored as int64 ns
}

func (f *fakeFlusher) ServerlessFlush(d time.Duration) {
	f.calls.Add(1)
	f.lastArg.Store(int64(d))
	if f.block > 0 {
		time.Sleep(f.block)
	}
}

// recordingDemux satisfies aggregator.Demultiplexer (via the embedded nil
// *aggregator.AgentDemultiplexer, which provides method promotion to fill out
// the interface) but overrides WaitForPendingSamples so Shutdown's drain
// phase reaches our hook. Calls to any other Demultiplexer method would
// dereference the nil embedded pointer — Shutdown never invokes them, but
// adding new call sites in Shutdown would surface here as a panic, which is
// the desired loud failure.
type recordingDemux struct {
	*aggregator.AgentDemultiplexer
	mu                sync.Mutex
	called            bool
	deadlineWasSet    bool
	timeUntilDeadline time.Duration
}

func (r *recordingDemux) WaitForPendingSamples(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	if dl, ok := ctx.Deadline(); ok {
		r.deadlineWasSet = true
		r.timeUntilDeadline = time.Until(dl)
	}
	return nil
}

// TestShutdownNilSafe verifies the documented nil-safety contract: Shutdown
// with both nil agent and nil flusher is a no-op (returns immediately, panics
// nowhere). This lets callers wire Shutdown in unconditionally regardless of
// API-key gating or partial initialization.
func TestShutdownNilSafe(t *testing.T) {
	// Should not panic, should return promptly.
	done := make(chan struct{})
	go func() {
		Shutdown(nil, nil, 100*time.Millisecond, 100*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown(nil, nil, ...) did not return promptly")
	}
}

// TestShutdownNilFlusherStillDrainsAgent verifies that a nil flusher skips
// the flush phase but the drain phase still runs against the agent.
func TestShutdownNilFlusherStillDrainsAgent(t *testing.T) {
	rd := &recordingDemux{}
	agent := &ServerlessMetricAgent{Demux: rd}

	Shutdown(agent, nil, 100*time.Millisecond, 50*time.Millisecond)

	rd.mu.Lock()
	defer rd.mu.Unlock()
	require.True(t, rd.called, "drain phase must run when flusher is nil")
	require.True(t, rd.deadlineWasSet, "drain phase must pass a context with a deadline")
}

// TestShutdownNilAgentStillFlushes verifies that a nil agent skips the
// drain phase but the flush phase still runs.
func TestShutdownNilAgentStillFlushes(t *testing.T) {
	f := &fakeFlusher{}
	Shutdown(nil, f, 100*time.Millisecond, 50*time.Millisecond)
	require.Equal(t, int64(1), f.calls.Load(), "flush phase must run when agent is nil")
	require.Equal(t, int64(0), f.lastArg.Load(), "Shutdown must invoke ServerlessFlush with zero timeout (external timer bounds the call)")
}

// TestShutdownFlushTimeoutBounded verifies the core guarantee of the flush
// phase: a ServerlessFlush that blocks past flushTimeout must not delay
// Shutdown's return. Without the external timer, a stuck DogStatsD flush
// would consume the entire shutdown grace window — the bug this helper
// exists to prevent.
func TestShutdownFlushTimeoutBounded(t *testing.T) {
	const flushTimeout = 50 * time.Millisecond
	// Block long enough that any failure to bound the flush phase is
	// obvious (orders of magnitude past flushTimeout).
	f := &fakeFlusher{block: 5 * time.Second}

	start := time.Now()
	Shutdown(nil, f, flushTimeout, 10*time.Millisecond)
	elapsed := time.Since(start)

	require.Equal(t, int64(1), f.calls.Load(), "flusher must be invoked exactly once")
	// Allow generous headroom for slow CI; the assertion is that we
	// returned in roughly flushTimeout rather than the 5s block.
	require.Less(t, elapsed, time.Second,
		"Shutdown must return within ~flushTimeout when ServerlessFlush blocks, got %v", elapsed)
}

// TestShutdownDrainTimeoutPropagates verifies the drain phase passes a
// context with a deadline derived from drainTimeout to
// ServerlessMetricAgent.Stop, so a stuck demultiplexer cannot exceed the
// configured drain budget.
func TestShutdownDrainTimeoutPropagates(t *testing.T) {
	const drainTimeout = 75 * time.Millisecond
	rd := &recordingDemux{}
	agent := &ServerlessMetricAgent{Demux: rd}

	Shutdown(agent, nil, 10*time.Millisecond, drainTimeout)

	rd.mu.Lock()
	defer rd.mu.Unlock()
	require.True(t, rd.called, "drain phase must run")
	require.True(t, rd.deadlineWasSet, "drain phase must pass a context with a deadline")
	// The deadline reported by recordingDemux is the remaining time at the
	// instant Stop was entered; it must be > 0 and <= drainTimeout. Allow
	// a small negative slack for clock jitter on slow CI.
	require.LessOrEqual(t, rd.timeUntilDeadline, drainTimeout,
		"deadline must not exceed drainTimeout, got %v vs %v", rd.timeUntilDeadline, drainTimeout)
	require.Greater(t, rd.timeUntilDeadline, -10*time.Millisecond,
		"deadline must not already be in the past, got %v", rd.timeUntilDeadline)
}
