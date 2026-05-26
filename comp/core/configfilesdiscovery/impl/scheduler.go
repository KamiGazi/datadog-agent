// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

package configfilesdiscoveryimpl

import (
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/scheduler"
)

const schedulerName = "configfiles-discovery"

type adScheduler struct {
	registry ingesterRegistry
	resolver targetResolver
	accessor accessorFactory
}

var _ scheduler.Scheduler = (*adScheduler)(nil)

func newADScheduler(registry ingesterRegistry, resolver targetResolver, accessor accessorFactory) *adScheduler {
	return &adScheduler{
		registry: registry,
		resolver: resolver,
		accessor: accessor,
	}
}

// Schedule is intentionally a no-op for the first TDD checkpoint.
func (s *adScheduler) Schedule(_ []integration.Config) {}

// Unschedule is intentionally a no-op for the first TDD checkpoint.
func (s *adScheduler) Unschedule(_ []integration.Config) {}

// Stop is intentionally a no-op for the first TDD checkpoint.
func (s *adScheduler) Stop() {}
