// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

// Package fx provides fx wiring for the config files discovery component.
package fx

import (
	configfilesdiscovery "github.com/DataDog/datadog-agent/comp/core/configfilesdiscovery/def"
	configfilesdiscoveryimpl "github.com/DataDog/datadog-agent/comp/core/configfilesdiscovery/impl"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
)

// Module defines the fx options for this component.
func Module() fxutil.Module {
	return fxutil.Component(
		fxutil.ProvideComponentConstructor(
			configfilesdiscoveryimpl.NewComponent,
		),
		fxutil.ProvideOptional[configfilesdiscovery.Component](),
	)
}
