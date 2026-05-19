// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package fakeintake

import "github.com/DataDog/datadog-agent/test/e2e-framework/common"

// DefaultRCSigningKeySeed is the default ed25519 seed used by fakeintake when
// WithRemoteConfig() is passed without an explicit seed. It is a fixed test-only
// key — never use it in production. All E2E tests share this seed so the TUF
// root JSON is deterministic and can be baked into the agent config at provision time.
const DefaultRCSigningKeySeed = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

type Params struct {
	LoadBalancerEnabled bool
	ImageURL            string
	CPU                 int
	Memory              int
	DDDevForwarding     bool
	RetentionPeriod     string
	RCSigningKeySeed    string // hex-encoded 32-byte ed25519 seed; empty disables fixed-key RC
}

type Option = func(*Params) error

// NewParams returns a new instance of Fakeintake Params
func NewParams(options ...Option) (*Params, error) {
	params := &Params{
		LoadBalancerEnabled: false,
		ImageURL:            "public.ecr.aws/datadog/fakeintake:latest",
		CPU:                 512,
		Memory:              1024,
		DDDevForwarding:     true,
		RetentionPeriod:     "15m",
	}
	return common.ApplyOption(params, options)
}

// WithLoadBalancer enable load balancer in front of the fakeintake
// Default is false
func WithLoadBalancer() Option {
	return func(p *Params) error {
		p.LoadBalancerEnabled = true
		return nil
	}
}

// WithImageURL sets the URL of the image to use to define the fakeintake
func WithImageURL(imageURL string) Option {
	return func(p *Params) error {
		p.ImageURL = imageURL
		return nil
	}
}

// WithCPU sets the number of CPU units to allocate to the fakeintake
// Default is 512 CPU units
func WithCPU(cpu int) Option {
	return func(p *Params) error {
		p.CPU = cpu
		return nil
	}
}

// WithMemory sets the amount (in MiB) of memory to allocate to the fakeintake
// Default is 1024 MiB
func WithMemory(memory int) Option {
	return func(p *Params) error {
		p.Memory = memory
		return nil
	}
}

// WithoutDDDevForwarding disables payload forwarding to dddev account.
// dddev forwarding is enabled by default
func WithoutDDDevForwarding() Option {
	return func(p *Params) error {
		p.DDDevForwarding = false
		return nil
	}
}

// WithRetentionPeriod set the retention period for the fakeintake
// Default is 15 minutes
// Possible values are: 1m, 10s, 1h
func WithRetentionPeriod(retentionPeriod string) Option {
	return func(p *Params) error {
		p.RetentionPeriod = retentionPeriod
		return nil
	}
}

// WithRemoteConfig enables fakeintake's Remote Config backend using the default
// fixed signing key seed (DefaultRCSigningKeySeed). The root JSON is deterministic
// so the agent's config_root / director_root can be set at provision time.
func WithRemoteConfig() Option {
	return func(p *Params) error {
		p.RCSigningKeySeed = DefaultRCSigningKeySeed
		return nil
	}
}

// WithRemoteConfigSigningKeySeed enables fakeintake's Remote Config backend with a
// specific hex-encoded 32-byte ed25519 seed instead of the default one.
func WithRemoteConfigSigningKeySeed(hexSeed string) Option {
	return func(p *Params) error {
		p.RCSigningKeySeed = hexSeed
		return nil
	}
}
