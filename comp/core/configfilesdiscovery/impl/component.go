// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

// Package configfilesdiscoveryimpl implements the configfilesdiscovery component.
package configfilesdiscoveryimpl

import (
	"context"

	"go.uber.org/fx"

	"github.com/DataDog/datadog-agent/comp/core/autodiscovery"
	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/scheduler"
	configfilesdiscovery "github.com/DataDog/datadog-agent/comp/core/configfilesdiscovery/def"
	compdef "github.com/DataDog/datadog-agent/comp/def"
)

// Requires defines the dependencies for the config files discovery component.
type Requires struct {
	Lifecycle     compdef.Lifecycle
	Autodiscovery autodiscovery.Component
}

// Provides defines the output of the config files discovery component.
type Provides struct {
	fx.Out

	Comp configfilesdiscovery.Component
}

type autodiscoverySchedulerRegistry interface {
	AddScheduler(string, scheduler.Scheduler, bool)
	RemoveScheduler(string)
}

type component struct {
	ac        autodiscoverySchedulerRegistry
	scheduler scheduler.Scheduler
}

func newComponent(
	ac autodiscoverySchedulerRegistry,
	registry ingesterRegistry,
	resolver targetResolver,
	accessor accessorFactory,
) *component {
	return &component{
		ac:        ac,
		scheduler: newADScheduler(registry, resolver, accessor),
	}
}

// NewComponent creates a no-op component skeleton for the first TDD checkpoint.
func NewComponent(reqs Requires) Provides {
	c := newComponent(
		reqs.Autodiscovery,
		noIngesterRegistry{},
		targetResolver{},
		noAccessorFactory{},
	)
	reqs.Lifecycle.Append(compdef.Hook{OnStart: c.start, OnStop: c.stop})
	return Provides{Comp: c}
}

func (c *component) start(_ context.Context) error {
	return nil
}

func (c *component) stop(_ context.Context) error {
	return nil
}
