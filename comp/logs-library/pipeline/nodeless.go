// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package pipeline

import (
	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
)

// applyRoutingHeaders sets x-dd-logs-routing on non-nodeless endpoints based on
// the primary transport, or strips it entirely for nodeless nodes.
func applyRoutingHeaders(endpoints *config.Endpoints, nodeless bool) {
	if nodeless {
		stripRoutingHeader(endpoints)
		return
	}

	var routingValue string
	if endpoints.UseGRPC {
		routingValue = "grpc"
	} else if endpoints.UseHTTP {
		routingValue = "http"
	} else {
		return
	}

	if endpoints.Main.ExtraHTTPHeaders == nil {
		endpoints.Main.ExtraHTTPHeaders = map[string]string{}
	}
	endpoints.Main.ExtraHTTPHeaders["x-dd-logs-routing"] = routingValue
	if len(endpoints.Endpoints) > 0 {
		if endpoints.Endpoints[0].ExtraHTTPHeaders == nil {
			endpoints.Endpoints[0].ExtraHTTPHeaders = map[string]string{}
		}
		endpoints.Endpoints[0].ExtraHTTPHeaders["x-dd-logs-routing"] = routingValue
	}
}

func stripRoutingHeader(endpoints *config.Endpoints) {
	delete(endpoints.Main.ExtraHTTPHeaders, "x-dd-logs-routing")
	for i := range endpoints.Endpoints {
		delete(endpoints.Endpoints[i].ExtraHTTPHeaders, "x-dd-logs-routing")
	}
}
