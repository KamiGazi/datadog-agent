// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package procmgrtest provides new-e2e helpers for asserting processes managed by
// dd-procmgr (describe output, /proc/<pid>/exe, optional expected binary paths).
package procmgrtest

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	DDOTProcessName = "datadog-agent-ddot"

	ddotProcmgrYAMLFileName = "datadog-agent-ddot.yaml"
	// StableDDOTProcmgrYAMLDeb is the stable processes.d DDOT file on classic deb/rpm installs.
	StableDDOTProcmgrYAMLDeb = "/opt/datadog-agent/processes.d/" + ddotProcmgrYAMLFileName
	// StableDDOTProcmgrYAMLOCI is the stable processes.d DDOT file on fleet OCI agent installs.
	StableDDOTProcmgrYAMLOCI = "/opt/datadog-packages/datadog-agent/stable/processes.d/" + ddotProcmgrYAMLFileName

	DDOTOtelAgentExtensionBinary            = "/opt/datadog-agent/ext/ddot/embedded/bin/otel-agent"
	DDOTOtelAgentFleetStableExtensionBinary = "/opt/datadog-packages/datadog-agent/stable/ext/ddot/embedded/bin/otel-agent"
	DDOTOtelAgentFleetPackageBinary         = "/opt/datadog-packages/datadog-agent-ddot/stable/embedded/bin/otel-agent"

	CLIBinDefault     = "/opt/datadog-agent/embedded/bin/dd-procmgr"
	CLIBinFleetStable = "/opt/datadog-packages/datadog-agent/stable/embedded/bin/dd-procmgr"

	procmgrDaemonRelPath     = "embedded/bin/dd-procmgrd"
	classicAgentInstallRoot  = "/opt/datadog-agent"
	fleetAgentStableRoot     = "/opt/datadog-packages/datadog-agent/stable"
	fleetAgentExperimentRoot = "/opt/datadog-packages/datadog-agent/experiment"

	ProcessStateRunning               = "Running"
	ProcessStateStopped               = "Stopped"
	waitForProcessTimeout             = 90 * time.Second
	waitForProcessPollInterval        = 2 * time.Second
	waitForProcessRunningStableWindow = 5 * time.Second
	waitForProcessRunningStablePoll   = 500 * time.Millisecond
)

// CLIBinForLinuxHost returns CLIBinFleetStable when that binary is executable on the host,
// otherwise CLIBinDefault (classic deb/rpm layout). Staging install-script suites often use
// /opt/datadog-agent only until an OCI experiment is promoted.
func CLIBinForLinuxHost(t *testing.T, executor CommandExecutor) string {
	t.Helper()
	cmd := fmt.Sprintf(`if sudo test -x %q; then echo %q; elif sudo test -x %q; then echo %q; else echo ""; fi`,
		CLIBinFleetStable, CLIBinFleetStable, CLIBinDefault, CLIBinDefault)
	out, err := executor.ExecuteCommand(cmd)
	require.NoError(t, err)
	path := strings.TrimSpace(out)
	require.NotEmpty(t, path, "dd-procmgr CLI not found (checked %s and %s)", CLIBinFleetStable, CLIBinDefault)
	return path
}

// LogProcmgrDaemonCandidates logs where dd-procmgrd exists on the host, mirroring
// service.ProcmgrDaemonAt / procmgrDaemonPresentOnHost and installer procmgrUsable checks.
func LogProcmgrDaemonCandidates(t *testing.T, executor CommandExecutor) {
	t.Helper()
	daemonRoots := []struct {
		label string
		root  string
	}{
		{"classic_deb_rpm", classicAgentInstallRoot},
		{"fleet_stable", fleetAgentStableRoot},
		{"fleet_experiment", fleetAgentExperimentRoot},
	}
	anyDaemon := false
	for _, c := range daemonRoots {
		daemonPath := c.root + "/" + procmgrDaemonRelPath
		out, err := executor.ExecuteCommand(fmt.Sprintf(
			`if sudo test -f %q; then echo present; else echo absent; fi`, daemonPath))
		if err != nil {
			t.Logf("dd-procmgrd %s (%s): check failed: %v", c.label, daemonPath, err)
			continue
		}
		state := strings.TrimSpace(out)
		t.Logf("dd-procmgrd %s (%s): %s", c.label, daemonPath, state)
		if state == "present" {
			anyDaemon = true
		}
	}
	findOut, err := executor.ExecuteCommand(
		`sudo find /opt/datadog-agent /opt/datadog-packages/datadog-agent -name 'dd-procmgrd' 2>/dev/null || true`)
	if err != nil {
		t.Logf("dd-procmgrd find: failed: %v", err)
	} else if found := strings.TrimSpace(findOut); found != "" {
		t.Logf("dd-procmgrd find:\n%s", found)
	} else {
		t.Logf("dd-procmgrd find: no dd-procmgrd under /opt/datadog-agent or /opt/datadog-packages/datadog-agent")
	}
	systemctlOut, err := executor.ExecuteCommand(`command -v systemctl >/dev/null 2>&1 && echo yes || echo no`)
	if err != nil {
		t.Logf("systemctl on PATH: check failed: %v", err)
	} else {
		t.Logf("systemctl on PATH: %s", strings.TrimSpace(systemctlOut))
	}
	for _, unit := range []string{"datadog-agent-procmgr.service", "datadog-agent.service"} {
		unitOut, err := executor.ExecuteCommand(fmt.Sprintf(
			`sudo systemctl show -p ActiveState,SubState,ConditionResult %q 2>/dev/null || echo "unit %s: not loaded"`, unit, unit))
		if err != nil {
			t.Logf("%s: %v", unit, err)
			continue
		}
		t.Logf("%s: %s", unit, strings.TrimSpace(unitOut))
	}
	// Mirrors procmgrBinaryExists(ctx, true) for deb hook vs deb hook after fleet remap gate.
	debRootDaemon, _ := executor.ExecuteCommand(fmt.Sprintf(
		`sudo test -f %q && echo yes || echo no`, classicAgentInstallRoot+"/"+procmgrDaemonRelPath))
	fleetGateDaemon, _ := executor.ExecuteCommand(fmt.Sprintf(
		`sudo test -f %q && echo yes || echo no`, fleetAgentStableRoot+"/"+procmgrDaemonRelPath))
	t.Logf("installer procmgrBinaryExists(deb ctx, stable): %s (root %s)", strings.TrimSpace(debRootDaemon), classicAgentInstallRoot)
	t.Logf("installer ddotExtensionProcmgrHookContext fleet gate (stable dd-procmgrd): %s", strings.TrimSpace(fleetGateDaemon))
	if strings.TrimSpace(fleetGateDaemon) == "yes" {
		ociRootDaemon, _ := executor.ExecuteCommand(fmt.Sprintf(
			`sudo test -f %q && echo yes || echo no`, fleetAgentStableRoot+"/"+procmgrDaemonRelPath))
		t.Logf("installer procmgrBinaryExists(oci stable ctx after remap, stable): %s (root %s)", strings.TrimSpace(ociRootDaemon), fleetAgentStableRoot)
	}
	if strings.TrimSpace(systemctlOut) == "yes" && anyDaemon {
		t.Logf("installer GetServiceManagerType would be ProcmgrType; procmgrUsable depends on procmgrBinaryExists after hook context remap")
	} else {
		t.Logf("installer GetServiceManagerType would not be ProcmgrType (systemctl=%s, anyDaemon=%v)", strings.TrimSpace(systemctlOut), anyDaemon)
	}
}

// LogStableDDOTProcmgrYAML logs known processes.d paths and any datadog-agent-ddot.yaml
// found under agent install roots (does not fail when the file is missing).
func LogStableDDOTProcmgrYAML(t *testing.T, executor CommandExecutor) {
	t.Helper()
	for _, path := range []string{StableDDOTProcmgrYAMLOCI, StableDDOTProcmgrYAMLDeb} {
		out, err := executor.ExecuteCommand(fmt.Sprintf(
			`if sudo test -f %q; then echo present; sudo cat %q; else echo absent; fi`, path, path))
		if err != nil {
			t.Logf("DDOT procmgr YAML %s: check failed: %v", path, err)
			continue
		}
		t.Logf("DDOT procmgr YAML %s: %s", path, strings.TrimSpace(out))
	}
	findOut, err := executor.ExecuteCommand(
		`sudo find /opt/datadog-agent /opt/datadog-packages/datadog-agent -name 'datadog-agent-ddot.yaml' 2>/dev/null || true`)
	if err != nil {
		t.Logf("DDOT procmgr YAML find: failed: %v", err)
	} else if found := strings.TrimSpace(findOut); found != "" {
		t.Logf("DDOT procmgr YAML find:\n%s", found)
	} else {
		t.Logf("DDOT procmgr YAML find: no datadog-agent-ddot.yaml under /opt/datadog-agent or /opt/datadog-packages/datadog-agent")
	}
	extFindOut, err := executor.ExecuteCommand(
		`sudo find /opt/datadog-agent /opt/datadog-packages/datadog-agent -path '*/ext/ddot/embedded/bin/otel-agent' 2>/dev/null || true`)
	if err != nil {
		t.Logf("DDOT extension binary find: failed: %v", err)
	} else if found := strings.TrimSpace(extFindOut); found != "" {
		t.Logf("DDOT extension binary find:\n%s", found)
	} else {
		t.Logf("DDOT extension binary find: no ext/ddot otel-agent under /opt/datadog-agent or /opt/datadog-packages/datadog-agent")
	}
	otelEnv, err := executor.ExecuteCommand(`echo "${DD_OTELCOLLECTOR_ENABLED:-unset}"`)
	if err != nil {
		t.Logf("DD_OTELCOLLECTOR_ENABLED: check failed: %v", err)
	} else {
		t.Logf("DD_OTELCOLLECTOR_ENABLED=%s", strings.TrimSpace(otelEnv))
	}
}

// StableDDOTProcmgrYAMLPath returns the on-disk path to stable DDOT procmgr YAML, preferring
// the fleet OCI layout when present, otherwise the classic deb/rpm path.
func StableDDOTProcmgrYAMLPath(t *testing.T, executor CommandExecutor) string {
	t.Helper()
	cmd := fmt.Sprintf(`if sudo test -f %q; then echo %q; elif sudo test -f %q; then echo %q; else echo ""; fi`,
		StableDDOTProcmgrYAMLOCI, StableDDOTProcmgrYAMLOCI, StableDDOTProcmgrYAMLDeb, StableDDOTProcmgrYAMLDeb)
	out, err := executor.ExecuteCommand(cmd)
	require.NoError(t, err)
	path := strings.TrimSpace(out)
	require.NotEmpty(t, path, "datadog-agent-ddot.yaml not found (checked %s and %s)", StableDDOTProcmgrYAMLOCI, StableDDOTProcmgrYAMLDeb)
	return path
}

// CommandExecutor executes a command on the remote host.
type CommandExecutor interface {
	ExecuteCommand(command string) (string, error)
}

type WaitForProcessArgs struct {
	ProcmgrCLIBin  string
	ProcessName    string
	ExpectedBinary string
	DesiredState   string
}

type WaitForProcessResult struct {
	PID      string
	Restarts int
}

// WaitForProcess polls dd-procmgr describe until State matches DesiredState and returns
// a snapshot result. Restarts is always populated; PID is only populated for
// ProcessStateRunning. For ProcessStateRunning, when ExpectedBinary is set, describe
// Command must equal it. The describe Command path is canonicalized with readlink -f and
// must match readlink -f /proc/<pid>/exe (PID from describe).
// For Running, success additionally requires State and PID to stay unchanged for
// waitForProcessRunningStableWindow inside one Eventually callback (otherwise the callback
// returns false and Eventually retries—e.g. a brief Running before a crash does not pass).
func WaitForProcess(t *testing.T, executor CommandExecutor, args WaitForProcessArgs) WaitForProcessResult {
	t.Helper()
	require.NotEmpty(t, args.ProcmgrCLIBin, "WaitForProcessArgs.ProcmgrCLIBin must be set")
	require.NotEmpty(t, args.DesiredState, "WaitForProcessArgs.DesiredState must be set")

	describeCmd := fmt.Sprintf(`sudo -u dd-agent -- %q describe %q`, args.ProcmgrCLIBin, args.ProcessName)
	desiredState := args.DesiredState

	var result WaitForProcessResult
	require.Eventually(t, func() bool {
		out, err := executor.ExecuteCommand(describeCmd)
		if err != nil {
			t.Logf("WaitForProcess: dd-procmgr describe cmd=%q err=%v\noutput:\n%s", describeCmd, err, out)
			return false
		}
		if st := fieldValue(out, "State"); st != desiredState {
			t.Logf("WaitForProcess: dd-procmgr describe cmd=%q State=%q (want %s)\noutput:\n%s", describeCmd, st, desiredState, out)
			return false
		}
		descCmd := fieldValue(out, "Command")
		if args.ExpectedBinary != "" {
			if descCmd != args.ExpectedBinary {
				t.Logf("WaitForProcess: dd-procmgr describe cmd=%q unexpected Command field got=%q want=%q\noutput:\n%s", describeCmd, descCmd, args.ExpectedBinary, out)
				return false
			}
		}
		if desiredState != ProcessStateRunning {
			result = WaitForProcessResult{
				Restarts: restartsFromDescribe(out),
			}
			return true
		}

		r, ok := resolveRunningPIDFromDescribe(t, executor, describeCmd, out)
		if !ok {
			return false
		}
		if !confirmStableRunningPID(t, executor, describeCmd, desiredState, r.PID) {
			return false
		}
		result = r
		return true
	}, waitForProcessTimeout, waitForProcessPollInterval, fmt.Sprintf("process %q should be %s via dd-procmgr describe", args.ProcessName, desiredState))
	return result
}

// WaitForDDOTRunning polls until process datadog-agent-ddot is Running, using CLIBinForLinuxHost
// and validating describe Command against expectedBinary (e.g. DDOTOtelAgentExtensionBinary vs
// DDOTOtelAgentFleetPackageBinary for extension vs standalone ddot-package installs).
func WaitForDDOTRunning(t *testing.T, executor CommandExecutor, expectedBinary string) WaitForProcessResult {
	t.Helper()
	cli := CLIBinForLinuxHost(t, executor)
	require.NotEmpty(t, expectedBinary,
		"expectedBinary must be set (use DDOTOtelAgentExtensionBinary, DDOTOtelAgentFleetStableExtensionBinary, DDOTOtelAgentFleetPackageBinary)")
	return WaitForProcess(t, executor, WaitForProcessArgs{
		ProcmgrCLIBin:  cli,
		ProcessName:    DDOTProcessName,
		ExpectedBinary: expectedBinary,
		DesiredState:   ProcessStateRunning,
	})
}

// confirmStableRunningPID returns true iff describe shows wantState and wantPID on every
// poll for waitForProcessRunningStableWindow. Any drift returns false so the caller can
// retry (e.g. DDOT may briefly report Running before a failed restart settles on a stable PID).
func confirmStableRunningPID(t *testing.T, executor CommandExecutor, describeCmd, wantState, wantPID string) bool {
	t.Helper()
	start := time.Now()
	for time.Since(start) < waitForProcessRunningStableWindow {
		out, err := executor.ExecuteCommand(describeCmd)
		if err != nil {
			t.Logf("confirmStableRunningPID: describe err=%v\n%s", err, out)
			return false
		}
		if st := fieldValue(out, "State"); st != wantState {
			t.Logf("confirmStableRunningPID: State=%q want %q", st, wantState)
			return false
		}
		if pid := fieldValue(out, "PID"); pid != wantPID {
			t.Logf("confirmStableRunningPID: PID=%q want %q", pid, wantPID)
			return false
		}
		time.Sleep(waitForProcessRunningStablePoll)
	}
	return true
}

func resolveRunningPIDFromDescribe(
	t *testing.T,
	executor CommandExecutor,
	describeCmd, describeOut string,
) (WaitForProcessResult, bool) {
	t.Helper()
	cmd, cmdExe, ok := resolveCommandExeFromDescribe(t, executor, describeCmd, describeOut)
	if !ok {
		return WaitForProcessResult{}, false
	}
	pid, ok := resolveRunningPIDFromProc(t, executor, describeCmd, describeOut, cmd, cmdExe)
	if !ok {
		return WaitForProcessResult{}, false
	}
	return WaitForProcessResult{
		PID:      pid,
		Restarts: restartsFromDescribe(describeOut),
	}, true
}

func resolveCommandExeFromDescribe(
	t *testing.T,
	executor CommandExecutor,
	describeCmd, describeOut string,
) (string, string, bool) {
	t.Helper()
	cmd := fieldValue(describeOut, "Command")
	cmdExe, err := executor.ExecuteCommand(fmt.Sprintf("sudo readlink -f %q", cmd))
	if err != nil {
		t.Logf("resolveCommandExeFromDescribe: describe cmd=%q readlink -f %q (Command from describe) err=%v\n%s", describeCmd, cmd, err, cmdExe)
		return "", "", false
	}
	cmdExe = strings.TrimSpace(cmdExe)
	if cmdExe == "" {
		t.Logf("resolveCommandExeFromDescribe: describe cmd=%q readlink -f %q returned empty path (Command from describe)", describeCmd, cmd)
		return "", "", false
	}
	return cmd, cmdExe, true
}

func resolveRunningPIDFromProc(
	t *testing.T,
	executor CommandExecutor,
	describeCmd, describeOut, cmd, cmdExe string,
) (string, bool) {
	t.Helper()
	pid := fieldValue(describeOut, "PID")
	if pid == "" || pid == "-" {
		t.Logf("resolveRunningPIDFromProc: dd-procmgr describe cmd=%q missing PID (got %q)\noutput:\n%s", describeCmd, pid, describeOut)
		return "", false
	}
	exeOut, err := executor.ExecuteCommand("sudo readlink -f /proc/" + pid + "/exe")
	if err != nil {
		t.Logf("resolveRunningPIDFromProc: readlink -f /proc/%s/exe err=%v\n%s", pid, err, exeOut)
		return "", false
	}
	if strings.TrimSpace(exeOut) != cmdExe {
		t.Logf("resolveRunningPIDFromProc: readlink -f /proc/%s/exe got=%q want=%q (canonical Command %q from dd-procmgr describe)\ndd-procmgr describe output:\n%s", pid, strings.TrimSpace(exeOut), cmdExe, cmd, describeOut)
		return "", false
	}
	return pid, true
}

func restartsFromDescribe(describeOut string) int {
	n, err := strconv.Atoi(fieldValue(describeOut, "Restarts"))
	if err != nil {
		return 0
	}
	return n
}

func fieldValue(output, label string) string {
	needle := label + ":"
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, needle) {
			return strings.TrimSpace(trimmed[len(needle):])
		}
	}
	return ""
}
