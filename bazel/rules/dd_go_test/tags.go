// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026-present Datadog, Inc.

// This file is the package's only non-underscored source — it carries the
// flavor tag data the extension and //bazel/rules/dd_go_test/dump_tags both
// need, and crucially makes the package visible to `go mod tidy` and friends
// (which skip files starting with `_`). The extension itself lives in
// _gazelle_extension.go so its @gazelle deps don't leak into the root
// go.mod; Bazel stitches both files into one go_library via copy_file.

package dd_go_test

// flavorSpecificTags / commonTags / unitTestTags mirror their counterparts in
// bazel/flavors/defs.bzl (which in turn mirror tasks/build_tags.py).
// LINUX_ONLY_TAGS are included here unconditionally: at Gazelle generation
// time we don't know the target platform, and flavor_gotags()'s select()
// enforces the Linux-only restriction at build time.
// Kept in sync via //bazel/flavors:verify_flavor_tags.
var commonTags = []string{"grpcnotrace", "no_dynamic_plugins", "retrynotrace", "trivy_no_javadb"}

var unitTestTags = []string{"test"}

var flavorSpecificTags = map[string][]string{
	"base": {
		"cel", "clusterchecks", "consul", "containerd", "cri", "crio", "docker",
		"ec2", "etcd", "fargateprocess", "jetson", "jmx", "kubeapiserver",
		"kubelet", "ncm", "netcgo", "nvml", "oracle", "orchestrator", "otlp",
		"podman", "python", "sharedlibrarycheck", "systemd", "systemprobechecks",
		"trivy", "zk", "zlib", "zstd",
	},
	"dogstatsd": {"containerd", "docker", "kubelet", "podman", "zlib", "zstd"},
	"fips": {
		"cel", "consul", "containerd", "cri", "crio", "docker", "ec2", "etcd",
		"fargateprocess", "goexperiment.systemcrypto", "jetson", "jmx",
		"kubeapiserver", "kubelet", "ncm", "netcgo", "nvml", "oracle",
		"orchestrator", "otlp", "podman", "python", "requirefips",
		"sharedlibrarycheck", "systemd", "systemprobechecks", "trivy", "zk",
		"zlib", "zstd",
	},
	"heroku": {
		"bundle_installer", "consul", "etcd", "jmx", "ncm", "netcgo", "otlp",
		"python", "sharedlibrarycheck", "systemprobechecks", "zk", "zlib", "zstd",
	},
	"iot": {"jetson", "systemd", "zlib", "zstd"},
}

// FlavorUnitTestTags is the per-flavor tag set the extension uses to decide
// which dd_go_test variants apply to a package's srcs. Composed from
// flavorSpecificTags + commonTags + unitTestTags at package init. Exported so
// //bazel/rules/dd_go_test/dump_tags can serialize it for
// //bazel/flavors:verify_flavor_tags.
var FlavorUnitTestTags = func() map[string][]string {
	out := make(map[string][]string, len(flavorSpecificTags))
	for flavor, specific := range flavorSpecificTags {
		tags := make([]string, 0, len(specific)+len(commonTags)+len(unitTestTags))
		tags = append(tags, specific...)
		tags = append(tags, commonTags...)
		tags = append(tags, unitTestTags...)
		out[flavor] = tags
	}
	return out
}()
