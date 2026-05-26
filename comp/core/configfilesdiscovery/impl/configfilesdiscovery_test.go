// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

package configfilesdiscoveryimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/autodiscovery"
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/scheduler"
	"github.com/DataDog/datadog-agent/comp/core/config"
	log "github.com/DataDog/datadog-agent/comp/core/log/def"
	logmock "github.com/DataDog/datadog-agent/comp/core/log/mock"
	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
	workloadmetafxmock "github.com/DataDog/datadog-agent/comp/core/workloadmeta/fx-mock"
	workloadmetamock "github.com/DataDog/datadog-agent/comp/core/workloadmeta/mock"
	compdef "github.com/DataDog/datadog-agent/comp/def"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

func TestResolveTargetDetectsRuntime(t *testing.T) {
	tests := []struct {
		name       string
		setupStore func(t *testing.T) workloadmetamock.Mock
		config     integration.Config
		wantTarget Target
		wantOK     bool
	}{
		{
			name: "host process",
			config: integration.Config{
				Name:      "redis",
				ServiceID: "process://1234",
				Instances: []integration.Data{
					[]byte("{}"),
				},
			},
			wantTarget: Target{
				Integration: "redis",
				Runtime:     RuntimeHost,
				ServiceID:   "process://1234",
				PID:         1234,
			},
			wantOK: true,
		},
		{
			name: "standalone docker container",
			config: integration.Config{
				Name:      "redis",
				ServiceID: "docker://abc123",
				Instances: []integration.Data{
					[]byte("{}"),
				},
			},
			wantTarget: Target{
				Integration:      "redis",
				Runtime:          RuntimeDocker,
				ServiceID:        "docker://abc123",
				ContainerID:      "abc123",
				ContainerRuntime: "docker",
			},
			wantOK: true,
		},
		{
			name: "container with kubernetes pod owner",
			setupStore: func(t *testing.T) workloadmetamock.Mock {
				store := newWorkloadmetaMock(t)
				store.Set(&workloadmeta.KubernetesPod{
					EntityID: workloadmeta.EntityID{Kind: workloadmeta.KindKubernetesPod, ID: "pod-uid"},
					EntityMeta: workloadmeta.EntityMeta{
						Name:      "redis-0",
						Namespace: "default",
					},
				})
				store.Set(&workloadmeta.Container{
					EntityID: workloadmeta.EntityID{Kind: workloadmeta.KindContainer, ID: "abc123"},
					Runtime:  workloadmeta.ContainerRuntimeContainerd,
					Owner:    &workloadmeta.EntityID{Kind: workloadmeta.KindKubernetesPod, ID: "pod-uid"},
				})
				return store
			},
			config: integration.Config{
				Name:      "redis",
				ServiceID: "containerd://abc123",
				Instances: []integration.Data{
					[]byte("{}"),
				},
			},
			wantTarget: Target{
				Integration:      "redis",
				Runtime:          RuntimeKubernetes,
				ServiceID:        "containerd://abc123",
				ContainerID:      "abc123",
				ContainerRuntime: "containerd",
				PodName:          "redis-0",
				PodNamespace:     "default",
			},
			wantOK: true,
		},
		{
			name: "unsupported standalone container runtime",
			config: integration.Config{
				Name:      "redis",
				ServiceID: "containerd://abc123",
				Instances: []integration.Data{
					[]byte("{}"),
				},
			},
			wantOK: false,
		},
		{
			name: "malformed service id",
			config: integration.Config{
				Name:      "redis",
				ServiceID: "not-an-ad-service-id",
				Instances: []integration.Data{
					[]byte("{}"),
				},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store workloadmeta.Component
			if tt.setupStore != nil {
				store = tt.setupStore(t)
			}
			resolver := targetResolver{store: store}

			got, ok := resolver.Resolve(tt.config)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				tt.wantTarget.ConfigDigest = tt.config.Digest()
				assert.Equal(t, tt.wantTarget, got)
			}
		})
	}
}

func newWorkloadmetaMock(t *testing.T) workloadmetamock.Mock {
	return fxutil.Test[workloadmetamock.Mock](t, fx.Options(
		fx.Provide(func() config.Component { return config.NewMock(t) }),
		fx.Provide(func() log.Component { return logmock.New(t) }),
		workloadmetafxmock.MockModule(workloadmeta.NewParams()),
	))
}

func TestSchedulerDispatchesRegisteredIntegrationsOnly(t *testing.T) {
	ingester := &recordingIngester{}
	s := newADScheduler(
		newFakeRegistry(map[string]IntegrationIngester{"redis": ingester}),
		targetResolver{},
		fakeAccessorFactory{},
	)

	s.Schedule([]integration.Config{
		checkConfig("redis", "process://1234"),
		checkConfig("nginx", "process://5678"),
		{Name: "", ServiceID: "process://9999", Instances: []integration.Data{[]byte("{}")}},
		{Name: "redis", ServiceID: "process://9999", LogsConfig: []byte(`[{}]`)},
		{Name: "redis", ServiceID: "process://9999", ClusterCheck: true, Instances: []integration.Data{[]byte("{}")}},
	})

	require.Len(t, ingester.scheduled, 1)
	assert.Equal(t, "redis", ingester.scheduled[0].target.Integration)
	assert.Equal(t, RuntimeHost, ingester.scheduled[0].target.Runtime)
	assert.Equal(t, RuntimeHost, ingester.scheduled[0].access.Runtime())
}

func TestSchedulerContinuesAfterInvalidConfigInBatch(t *testing.T) {
	ingester := &recordingIngester{}
	s := newADScheduler(
		newFakeRegistry(map[string]IntegrationIngester{"redis": ingester}),
		targetResolver{},
		fakeAccessorFactory{},
	)

	s.Schedule([]integration.Config{
		checkConfig("redis", "not-an-ad-service-id"),
		checkConfig("redis", "docker://abc123"),
	})

	require.Len(t, ingester.scheduled, 1)
	assert.Equal(t, RuntimeDocker, ingester.scheduled[0].target.Runtime)
	assert.Equal(t, "abc123", ingester.scheduled[0].target.ContainerID)
}

func TestSchedulerUnschedulesActiveTargets(t *testing.T) {
	ingester := &recordingIngester{}
	cfg := checkConfig("redis", "docker://abc123")
	s := newADScheduler(
		newFakeRegistry(map[string]IntegrationIngester{"redis": ingester}),
		targetResolver{},
		fakeAccessorFactory{},
	)

	s.Schedule([]integration.Config{cfg})
	s.Unschedule([]integration.Config{cfg})

	require.Len(t, ingester.scheduled, 1)
	require.Len(t, ingester.unscheduled, 1)
	assert.Equal(t, ingester.scheduled[0].target, ingester.unscheduled[0])
}

func TestSchedulerSkipsUnscheduleForInactiveTargets(t *testing.T) {
	ingester := &recordingIngester{}
	s := newADScheduler(
		newFakeRegistry(map[string]IntegrationIngester{"redis": ingester}),
		targetResolver{},
		fakeAccessorFactory{},
	)

	s.Unschedule([]integration.Config{checkConfig("redis", "docker://abc123")})

	assert.Empty(t, ingester.unscheduled)
}

func TestComponentRegistersAutodiscoverySchedulerOnStart(t *testing.T) {
	ac := &fakeAutodiscovery{}
	lifecycle := &recordingLifecycle{}

	NewComponent(Requires{
		Lifecycle:     lifecycle,
		Autodiscovery: ac,
	})

	require.NotNil(t, lifecycle.hook.OnStart)
	require.NoError(t, lifecycle.hook.OnStart(context.Background()))
	assert.Equal(t, schedulerName, ac.addedName)
	assert.True(t, ac.replay)
	require.Implements(t, (*scheduler.Scheduler)(nil), ac.scheduler)

	require.NotNil(t, lifecycle.hook.OnStop)
	require.NoError(t, lifecycle.hook.OnStop(context.Background()))
	assert.Equal(t, schedulerName, ac.removedName)
}

func checkConfig(name string, serviceID string) integration.Config {
	return integration.Config{
		Name:      name,
		ServiceID: serviceID,
		Instances: []integration.Data{
			[]byte("{}"),
		},
	}
}

type fakeRegistry struct {
	ingesters map[string]IntegrationIngester
}

func newFakeRegistry(ingesters map[string]IntegrationIngester) fakeRegistry {
	return fakeRegistry{ingesters: ingesters}
}

func (r fakeRegistry) Get(integration string) (IntegrationIngester, bool) {
	ingester, found := r.ingesters[integration]
	return ingester, found
}

type recordingLifecycle struct {
	hook compdef.Hook
}

func (l *recordingLifecycle) Append(hook compdef.Hook) {
	l.hook = hook
}

type fakeAccessorFactory struct{}

func (fakeAccessorFactory) ForTarget(target Target) (RuntimeAccessor, bool) {
	return fakeAccessor{runtime: target.Runtime}, true
}

type fakeAccessor struct {
	runtime RuntimeType
}

func (a fakeAccessor) Runtime() RuntimeType {
	return a.runtime
}

type recordingIngester struct {
	scheduled   []scheduledCall
	unscheduled []Target
}

type scheduledCall struct {
	target Target
	access RuntimeAccessor
}

func (i *recordingIngester) Schedule(_ context.Context, target Target, access RuntimeAccessor) error {
	i.scheduled = append(i.scheduled, scheduledCall{
		target: target,
		access: access,
	})
	return nil
}

func (i *recordingIngester) Unschedule(_ context.Context, target Target) error {
	i.unscheduled = append(i.unscheduled, target)
	return nil
}

type fakeAutodiscovery struct {
	autodiscovery.Component

	addedName   string
	removedName string
	scheduler   scheduler.Scheduler
	replay      bool
}

func (a *fakeAutodiscovery) AddScheduler(name string, scheduler scheduler.Scheduler, replay bool) {
	a.addedName = name
	a.scheduler = scheduler
	a.replay = replay
}

func (a *fakeAutodiscovery) RemoveScheduler(name string) {
	a.removedName = name
}
