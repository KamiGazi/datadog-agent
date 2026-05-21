// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build kubelet

package pipeline

import (
	"context"

	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/hostinfo"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

// isNodelessNode returns true when the current node has the label class=nodeless.
func isNodelessNode(_ pkgconfigmodel.Reader) bool {
	nodeInfo, err := hostinfo.NewNodeInfo()
	if err != nil {
		log.Debugf("logs-agent: could not create NodeInfo to check nodeless label: %v", err)
		return false
	}

	labels, err := nodeInfo.GetNodeLabels(context.Background())
	if err != nil {
		log.Debugf("logs-agent: could not get node labels: %v", err)
		return false
	}

	return labels["class"] == "nodeless"
}
