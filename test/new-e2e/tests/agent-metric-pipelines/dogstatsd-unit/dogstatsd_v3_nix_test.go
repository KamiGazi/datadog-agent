// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package dogstatsdunit

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/test/e2e-framework/components/datadog/agentparams"
	scenec2 "github.com/DataDog/datadog-agent/test/e2e-framework/scenarios/aws/ec2"
	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/e2e"
	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/environments"
	awshost "github.com/DataDog/datadog-agent/test/e2e-framework/testing/provisioners/aws/host"
	"github.com/DataDog/datadog-agent/test/fakeintake/aggregator"
)

const (
	v3CountMetric = "e2e.metric.v3.count"

	v3HistogramMetric    = "e2e.metric.v3.histogram"
	v3HistogramMaxSuffix = ".max"

	v3TimingMetric    = "e2e.metric.v3.timing"
	v3TimingMaxSuffix = ".max"

	expectedV3TimingUnit = "millisecond"
)

type dogstatsdV3Suite struct {
	e2e.BaseSuite[environments.Host]
}

// TestDogstatsdV3 verifies that when the V3 metrics intake API is enabled the agent sends
// DogStatsD metrics to /api/intake/metrics/v3/series instead of /api/v2/series.
func TestDogstatsdV3(t *testing.T) {
	t.Parallel()

	agentOptions := []agentparams.Option{
		agentparams.WithAgentConfig(`
histogram_aggregates:
  - max
  - avg
  - count
histogram_percentiles:
  - "0.95"
`),
		agentparams.WithV3MetricsEnabled(),
	}

	e2e.Run(t, &dogstatsdV3Suite{},
		e2e.WithProvisioner(
			awshost.Provisioner(
				awshost.WithRunOptions(
					// WithV3MetricsEnabled must be listed after WithFakeintake, which the
					// provisioner prepends automatically, so this ordering is correct.
					scenec2.WithAgentOptions(agentOptions...),
				),
			),
		),
		e2e.WithStackName("dogstatsdv3"),
	)
}

func (s *dogstatsdV3Suite) sendMetric(name string, value float32, metricType string) {
	cmd := fmt.Sprintf(`bash -c 'echo -n "%s:%f|%s" > /dev/udp/127.0.0.1/8125'`, name, value, metricType)
	s.Env().RemoteHost.MustExecute(cmd)
}

// TestMetricsReachV3Endpoint sends multiple DogStatsD metric types and asserts that they
// reach the V3 intake endpoint in fakeintake and do NOT appear on the V2 endpoint.
func (s *dogstatsdV3Suite) TestMetricsReachV3Endpoint() {
	require.NoError(s.T(), s.Env().FakeIntake.Client().FlushServerAndResetAggregators())

	// Keep sending until all metric types appear on the V3 endpoint with the expected units.
	require.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); s.sendMetric(v3CountMetric, 1, "c") }()
		go func() { defer wg.Done(); s.sendMetric(v3HistogramMetric, 100, "h") }()
		go func() {
			defer wg.Done()
			s.sendMetric(v3TimingMetric, 100, "ms")
			s.sendMetric(v3TimingMetric, 0.2, "ms")
			s.sendMetric(v3TimingMetric, 3000, "ms")
		}()
		wg.Wait()

		v3Count, err := s.Env().FakeIntake.Client().FilterMetricsV3(v3CountMetric)
		assert.NoError(c, err)
		assertValidV3Metric(c, v3Count, "", func(value float64) bool {
			return value > 0
		}, "counter must reach V3 endpoint (/api/intake/metrics/v3/series) without a unit")

		v3Histogram, err := s.Env().FakeIntake.Client().FilterMetricsV3(v3HistogramMetric + v3HistogramMaxSuffix)
		assert.NoError(c, err)
		assertValidV3Metric(c, v3Histogram, "", func(value float64) bool {
			return value == 100
		}, "histogram .max must reach V3 endpoint (/api/intake/metrics/v3/series) without a unit")

		v3Timing, err := s.Env().FakeIntake.Client().FilterMetricsV3(v3TimingMetric + v3TimingMaxSuffix)
		assert.NoError(c, err)
		assertValidV3Metric(c, v3Timing, expectedV3TimingUnit, func(value float64) bool {
			return value == 3000
		}, "timing .max must reach V3 endpoint (/api/intake/metrics/v3/series) with unit millisecond")
	}, 2*time.Minute, 5*time.Second, "timed out waiting for all metric types on V3 endpoint")

	// Confirm that nothing leaked to the V2 endpoint for the same metric names.
	v2Count, err := s.Env().FakeIntake.Client().FilterMetrics(v3CountMetric)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), v2Count, "counter must NOT appear on V2 endpoint when V3 is enabled")

	v2Histogram, err := s.Env().FakeIntake.Client().FilterMetrics(v3HistogramMetric + v3HistogramMaxSuffix)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), v2Histogram, "histogram must NOT appear on V2 endpoint when V3 is enabled")

	v2Timing, err := s.Env().FakeIntake.Client().FilterMetrics(v3TimingMetric + v3TimingMaxSuffix)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), v2Timing, "timing must NOT appear on V2 endpoint when V3 is enabled")
}

func assertValidV3Metric(c *assert.CollectT, metrics []*aggregator.MetricSeriesV3, expectedUnit string, valueMatches func(float64) bool, message string) {
	if !assert.NotEmpty(c, metrics, message) {
		return
	}
	hasMatchingPoint := false
	for _, metric := range metrics {
		assert.Equal(c, expectedUnit, metric.Unit,
			"metric %q must have unit %q, got %q", metric.Metric, expectedUnit, metric.Unit)
		for _, point := range metric.Points {
			if valueMatches(point.Value) {
				hasMatchingPoint = true
			}
		}
	}
	assert.Truef(c, hasMatchingPoint, "expected a matching point in %#v", metrics)
}
