// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package remoteconfig

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/e2e"
	"github.com/DataDog/datadog-agent/test/e2e-framework/testing/environments"
	awshost "github.com/DataDog/datadog-agent/test/e2e-framework/testing/provisioners/aws/host"
)

type logLevelSuite struct {
	e2e.BaseSuite[environments.Host]
}

func TestLogLevelRCSuite(t *testing.T) {
	t.Parallel()
	e2e.Run(t, &logLevelSuite{},
		e2e.WithProvisioner(awshost.Provisioner()),
	)
}

// TestLogLevelRC verifies that the agent starts at info level (no debug logs), then
// transitions to debug level after a Remote Config AGENT_CONFIG payload is pushed.
func (s *logLevelSuite) TestLogLevelRC() {
	t := s.T()
	rh := s.Env().RemoteHost
	fi := s.Env().FakeIntake.Client()

	// Wait for agent ready before reading logs.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, s.Env().Agent.Client.IsReady())
	}, 2*time.Minute, 5*time.Second, "agent did not become ready")

	// Confirm no DEBUG lines exist at default (info) log level.
	agentLog, err := rh.ReadFilePrivileged("/var/log/datadog/agent.log")
	require.NoError(t, err)
	require.False(t, strings.Contains(string(agentLog), "| DEBUG |"),
		"expected no DEBUG logs at default log level")

	// Push an AGENT_CONFIG layer that sets log_level to debug.
	err = fi.RCAddConfig("", "AGENT_CONFIG", "layer1", "log_level_debug",
		[]byte(`{"name":"layer1","config":{"log_level":"debug"}}`))
	require.NoError(t, err)

	// Push the configuration_order so the agent applies the layer.
	err = fi.RCAddConfig("", "AGENT_CONFIG", "configuration_order", "order",
		[]byte(`{"order":["layer1"],"internal_order":[]}`))
	require.NoError(t, err)

	// Wait until debug logs appear in the agent log file.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logs, err := rh.ReadFilePrivileged("/var/log/datadog/agent.log")
		assert.NoError(c, err)
		assert.True(c, strings.Contains(string(logs), "| DEBUG |"),
			"expected DEBUG logs after RC log level change")
	}, 3*time.Minute, 10*time.Second, "agent did not produce debug logs after RC log level change")
}
