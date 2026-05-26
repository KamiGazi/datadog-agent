// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

package configfilesdiscoveryimpl

import (
	"context"

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

type noIngesterRegistry struct{}

func (noIngesterRegistry) Get(string) (IntegrationIngester, bool) {
	return nil, false
}

type noAccessorFactory struct{}

func (noAccessorFactory) ForTarget(Target) (RuntimeAccessor, bool) {
	return nil, false
}

type targetResolver struct {
	store workloadmetaStore
}

type workloadmetaStore interface {
	GetContainer(string) (*workloadmeta.Container, error)
	GetKubernetesPodForContainer(string) (*workloadmeta.KubernetesPod, error)
}

// Resolve is intentionally a placeholder for the first TDD checkpoint.
func (r targetResolver) Resolve(_ integration.Config) (Target, bool) {
	return Target{}, false
}
