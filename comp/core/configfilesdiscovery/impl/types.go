// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

package configfilesdiscoveryimpl

import (
	"context"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/integration"
	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
)

// RuntimeType identifies where an integration's backing service is running.
type RuntimeType string

const (
	// RuntimeKubernetes identifies a service running in a Kubernetes pod.
	RuntimeKubernetes RuntimeType = "k8s"
	// RuntimeDocker identifies a service running in a standalone Docker container.
	RuntimeDocker RuntimeType = "docker"
	// RuntimeHost identifies a service running directly on the host.
	RuntimeHost RuntimeType = "host"
)

// Target is the normalized ingestion target derived from an AD scheduled config.
type Target struct {
	Integration      string
	ConfigDigest     string
	Runtime          RuntimeType
	ServiceID        string
	ContainerID      string
	ContainerRuntime string
	PodName          string
	PodNamespace     string
	PID              int
}

// RuntimeAccessor is the runtime-specific access layer used by ingesters.
type RuntimeAccessor interface {
	Runtime() RuntimeType
}

// IntegrationIngester handles config ingestion for one integration.
type IntegrationIngester interface {
	Schedule(context.Context, Target, RuntimeAccessor) error
	Unschedule(context.Context, Target) error
}

type ingesterRegistry interface {
	Get(string) (IntegrationIngester, bool)
}

type accessorFactory interface {
	ForTarget(Target) (RuntimeAccessor, bool)
}

type runtimeAccessorFactory struct{}

func (runtimeAccessorFactory) ForTarget(target Target) (RuntimeAccessor, bool) {
	if target.Runtime == "" {
		return nil, false
	}
	return runtimeAccessor{runtime: target.Runtime}, true
}

type runtimeAccessor struct {
	runtime RuntimeType
}

func (a runtimeAccessor) Runtime() RuntimeType {
	return a.runtime
}

type noIngesterRegistry struct{}

func (noIngesterRegistry) Get(string) (IntegrationIngester, bool) {
	return nil, false
}

type targetResolver struct {
	store workloadmetaStore
}

type workloadmetaStore interface {
	GetContainer(string) (*workloadmeta.Container, error)
	GetKubernetesPodForContainer(string) (*workloadmeta.KubernetesPod, error)
}

func (r targetResolver) Resolve(config integration.Config) (Target, bool) {
	if config.Name == "" || config.ServiceID == "" || !config.IsCheckConfig() {
		return Target{}, false
	}

	runtime, id, ok := parseServiceID(config.ServiceID)
	if !ok {
		return Target{}, false
	}

	target := Target{
		Integration:  config.Name,
		ConfigDigest: config.Digest(),
		ServiceID:    config.ServiceID,
	}

	switch runtime {
	case "process":
		pid, err := strconv.Atoi(id)
		if err != nil {
			return Target{}, false
		}
		target.Runtime = RuntimeHost
		target.PID = pid
		return target, true
	case "docker":
		target.Runtime = RuntimeDocker
		target.ContainerID = id
		target.ContainerRuntime = runtime
	default:
		target.ContainerID = id
		target.ContainerRuntime = runtime
	}

	if r.store == nil {
		return target, target.Runtime == RuntimeDocker
	}

	container, err := r.store.GetContainer(id)
	if err == nil && container != nil && container.Runtime != "" {
		target.ContainerRuntime = string(container.Runtime)
	}

	pod, err := r.store.GetKubernetesPodForContainer(id)
	if err != nil || pod == nil {
		return target, target.Runtime == RuntimeDocker
	}

	target.Runtime = RuntimeKubernetes
	target.PodName = pod.Name
	target.PodNamespace = pod.Namespace
	return target, true
}

func parseServiceID(serviceID string) (string, string, bool) {
	runtime, id, found := strings.Cut(serviceID, "://")
	if !found || runtime == "" || id == "" {
		return "", "", false
	}
	return runtime, id, true
}
