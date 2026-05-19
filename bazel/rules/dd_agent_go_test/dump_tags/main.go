// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

// dump_tags writes the Gazelle extension's FlavorUnitTestTags to stdout as
// "<flavor>\t<tag>" lines. //bazel/flavors:verify_flavor_tags subprocesses it
// to assert the composed per-flavor tag sets match the Python and Starlark
// sources, without depending on the Go file's internal layout.
package main

import (
	"bufio"
	"fmt"
	"os"

	ddagentgotest "github.com/DataDog/datadog-agent/bazel/rules/dd_agent_go_test"
)

func main() {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for flavor, tags := range ddagentgotest.FlavorUnitTestTags {
		for _, tag := range tags {
			fmt.Fprintf(w, "%s\t%s\n", flavor, tag)
		}
	}
}
