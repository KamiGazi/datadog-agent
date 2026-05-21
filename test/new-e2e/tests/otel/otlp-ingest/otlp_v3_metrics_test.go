// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package otlpingest

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/test/e2e-framework/components/datadog/apps"
	"github.com/DataDog/datadog-agent/test/e2e-framework/components/datadog/dockeragentparams"
	"github.com/DataDog/datadog-agent/test/e2e-framework/scenarios/aws/ec2docker"
	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/e2e"
	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/environments"
	awsdocker "github.com/DataDog/datadog-agent/test/e2e-framework/testing/provisioners/aws/docker"
	"github.com/DataDog/datadog-agent/test/fakeintake/aggregator"
	fakeintakeclient "github.com/DataDog/datadog-agent/test/fakeintake/client"
	"github.com/DataDog/datadog-agent/test/new-e2e/tests/otel/utils"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const otlpV3MetricName = "calendar-rest-go.api.counter"

type otlpV3MetricsDockerTestSuite struct {
	e2e.BaseSuite[environments.DockerHost]
}

// TestOTLPV3MetricsDocker verifies that OTLP metrics ingested by the Agent are sent to
// /api/intake/metrics/v3/series instead of /api/v2/series when V3 metrics intake is enabled.
func TestOTLPV3MetricsDocker(t *testing.T) {
	t.Parallel()

	e2e.Run(t,
		&otlpV3MetricsDockerTestSuite{},
		e2e.WithProvisioner(
			awsdocker.Provisioner(
				awsdocker.WithRunOptions(
					ec2docker.WithAgentOptions(
						dockeragentparams.WithLogs(),
						dockeragentparams.WithV3MetricsEnabled(),
						dockeragentparams.WithAgentServiceEnvVariable("DD_OTLP_CONFIG_RECEIVER_PROTOCOLS_GRPC_ENDPOINT", pulumi.StringPtr("0.0.0.0:4317")),
						dockeragentparams.WithAgentServiceEnvVariable("DD_OTLP_CONFIG_RECEIVER_PROTOCOLS_HTTP_ENDPOINT", pulumi.StringPtr("0.0.0.0:4318")),
						dockeragentparams.WithAgentServiceEnvVariable("DD_LOGS_ENABLED", pulumi.StringPtr("true")),
						dockeragentparams.WithAgentServiceEnvVariable("DD_OTLP_CONFIG_LOGS_ENABLED", pulumi.StringPtr("true")),
						dockeragentparams.WithAgentServiceEnvVariable("DD_LOGS_CONFIG_CONTAINER_COLLECT_ALL", pulumi.StringPtr("false")),
						dockeragentparams.WithAgentServiceEnvVariable("DD_OTLP_CONFIG_METRICS_RESOURCE_ATTRIBUTES_AS_TAGS", pulumi.StringPtr("true")),
						dockeragentparams.WithExtraComposeManifest("calendar-rest-go", pulumi.String(strings.ReplaceAll(otlpIngestCompose, "{APPS_VERSION}", apps.Version))),
					),
				),
			),
		),
		e2e.WithStackName("otlpv3metrics"),
	)
}

func (s *otlpV3MetricsDockerTestSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	defer s.CleanupOnSetupFailure()

	utils.TestCalendarAppDocker(s)
}

func (s *otlpV3MetricsDockerTestSuite) TestOTLPMetricsReachV3Endpoint() {
	require.NoError(s.T(), s.Env().FakeIntake.Client().FlushServerAndResetAggregators())

	serviceTag := "service:" + utils.CalendarService

	var v3Metrics []*aggregator.MetricSeriesV3
	require.EventuallyWithT(s.T(), func(c *assert.CollectT) {
		var err error
		v3Metrics, err = s.Env().FakeIntake.Client().FilterMetricsV3(
			otlpV3MetricName,
			fakeintakeclient.WithTags[*aggregator.MetricSeriesV3]([]string{serviceTag}),
		)
		assert.NoError(c, err)
		if !assert.NotEmpty(c, v3Metrics, "OTLP counter must reach V3 endpoint (/api/intake/metrics/v3/series)") {
			return
		}
		for _, metric := range v3Metrics {
			assert.NotEmpty(c, metric.Points, "V3 series must contain decoded points")
			for _, point := range metric.Points {
				assert.NotZero(c, point.Timestamp, "V3 points must carry timestamps")
				assert.Greater(c, point.Value, 0.0, "OTLP counter points must carry positive values")
			}
		}
	}, 5*time.Minute, 10*time.Second, "timed out waiting for OTLP metrics on V3 endpoint")

	v2Metrics, err := s.Env().FakeIntake.Client().FilterMetrics(otlpV3MetricName)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), v2Metrics, "OTLP metrics must NOT appear on V2 endpoint (/api/v2/series) when V3 is enabled")

	require.NotEmpty(s.T(), v3Metrics)
	assert.Contains(s.T(), v3Metrics[0].Tags, serviceTag, "V3 series must carry service tag")
}
