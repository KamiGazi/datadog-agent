// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

package configfilesdiscoveryimpl

import (
	"context"
	"sync"

	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/scheduler"
)

const schedulerName = "configfiles-discovery"

type adScheduler struct {
	registry ingesterRegistry
	resolver targetResolver
	accessor accessorFactory

	mu     sync.Mutex
	active map[string]activeTarget
}

type activeTarget struct {
	target   Target
	ingester IntegrationIngester
}

var _ scheduler.Scheduler = (*adScheduler)(nil)

func newADScheduler(registry ingesterRegistry, resolver targetResolver, accessor accessorFactory) *adScheduler {
	return &adScheduler{
		registry: registry,
		resolver: resolver,
		accessor: accessor,
		active:   make(map[string]activeTarget),
	}
}

func (s *adScheduler) Schedule(configs []integration.Config) {
	for _, config := range configs {
		target, ok := s.resolver.Resolve(config)
		if !ok {
			continue
		}

		ingester, ok := s.registry.Get(target.Integration)
		if !ok {
			continue
		}

		accessor, ok := s.accessor.ForTarget(target)
		if !ok {
			continue
		}

		if err := ingester.Schedule(context.Background(), target, accessor); err != nil {
			continue
		}

		s.mu.Lock()
		s.active[target.ConfigDigest] = activeTarget{
			target:   target,
			ingester: ingester,
		}
		s.mu.Unlock()
	}
}

func (s *adScheduler) Unschedule(configs []integration.Config) {
	for _, config := range configs {
		digest := config.Digest()

		s.mu.Lock()
		active, ok := s.active[digest]
		if ok {
			delete(s.active, digest)
		}
		s.mu.Unlock()

		if !ok {
			continue
		}

		_ = active.ingester.Unschedule(context.Background(), active.target)
	}
}

// Stop is intentionally a no-op for the first TDD checkpoint.
func (s *adScheduler) Stop() {}
