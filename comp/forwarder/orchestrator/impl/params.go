// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package orchestratorimpl

import (
	compdef "github.com/DataDog/datadog-agent/comp/def"
	orchestrator "github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/def"
)

// paramsProvides wraps Params in a compdef.Out struct so ProvideComponentConstructor
// can provide it — Params itself has unexported fields which ProvideComponentConstructor rejects.
type paramsProvides struct {
	compdef.Out
	Params orchestrator.Params
}
