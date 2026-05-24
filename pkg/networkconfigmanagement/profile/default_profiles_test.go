// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

//go:build test

package profile

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func Test_DefaultProfiles_Running(t *testing.T) {
	tests := []struct {
		name                      string
		profile                   *NCMProfile
		fixture                   Fixture
		expectedExtractedMetadata *ExtractedMetadata
		expectedErrMsg            string
	}{

		{
			name:                      "Cisco ASA",
			profile:                   DefaultProfile("cisco-asa"),
			fixture:                   loadFixture("cisco-asa", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:    "Cisco IOS",
			profile: DefaultProfile("cisco-ios"),
			fixture: loadFixture("cisco-ios", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp:  1760099696,
				ConfigSize: 3781,
			},
		},
		{
			name:    "JunOS",
			profile: DefaultProfile("junos"),
			fixture: loadFixture("junos", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{
				ConfigSize: 0,
				Timestamp:  1730646727,
				Author:     "netops",
			},
		},
		{
			name:                      "PAN-OS",
			profile:                   DefaultProfile("pan-os"),
			fixture:                   loadFixture("pan-os", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:                      "AOSW",
			profile:                   DefaultProfile("aosw"),
			fixture:                   loadFixture("aosw", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:    "NXOS",
			profile: DefaultProfile("nxos"),
			fixture: loadFixture("nxos", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp: 1767709263,
			},
		},
		{
			name:                      "TMOS",
			profile:                   DefaultProfile("tmos"),
			fixture:                   loadFixture("tmos", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:                      "AOSCX",
			profile:                   DefaultProfile("aoscx"),
			fixture:                   loadFixture("aoscx", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:                      "EOS",
			profile:                   DefaultProfile("eos"),
			fixture:                   loadFixture("eos", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:                      "fortios",
			profile:                   DefaultProfile("fortios"),
			fixture:                   loadFixture("fortios", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:    "DellOS10",
			profile: DefaultProfile("dellos10"),
			fixture: loadFixture("dellos10", "running"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp: 1491873902,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualOutput, actualExtractedMetadata, err := tt.profile.ProcessConfig(tt.fixture.Initial)
			if tt.expectedErrMsg != "" {
				assert.EqualError(t, err, tt.expectedErrMsg)
			}

			// use cmp.Diff for a nicer output if the strings don't match, but still assert that they are equal
			assert.Empty(t, cmp.Diff(string(tt.fixture.Expected), string(actualOutput)))
			assert.Equal(t, tt.expectedExtractedMetadata, actualExtractedMetadata)
		})
	}
}

func Test_DefaultProfiles_Startup(t *testing.T) {
	tests := []struct {
		name                      string
		profile                   *NCMProfile
		fixture                   Fixture
		expectedExtractedMetadata *ExtractedMetadata
		expectedErrMsg            string
	}{
		{
			name:    "Cisco IOS",
			profile: DefaultProfile("cisco-ios"),
			fixture: loadFixture("cisco-ios", "startup"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp:  1765307830,
				ConfigSize: 3163,
			},
		},
		{
			name:    "NXOS",
			profile: DefaultProfile("nxos"),
			fixture: loadFixture("nxos", "startup"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp: 1767899167,
			},
		},
		{
			name:                      "AOSCX",
			profile:                   DefaultProfile("aoscx"),
			fixture:                   loadFixture("aoscx", "startup"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
		{
			name:    "EOS",
			profile: DefaultProfile("eos"),
			fixture: loadFixture("eos", "startup"),
			expectedExtractedMetadata: &ExtractedMetadata{
				Timestamp: 1392798871,
				Author:    "admin",
			},
		},
		{
			name:                      "dellos10",
			profile:                   DefaultProfile("dellos10"),
			fixture:                   loadFixture("dellos10", "startup"),
			expectedExtractedMetadata: &ExtractedMetadata{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualOutput, actualExtractedMetadata, err := tt.profile.ProcessConfig(tt.fixture.Initial)
			if tt.expectedErrMsg != "" {
				assert.EqualError(t, err, tt.expectedErrMsg)
			}

			// use cmp.Diff for a nicer output if the strings don't match, but still assert that they are equal
			assert.Empty(t, cmp.Diff(string(tt.fixture.Expected), string(actualOutput)))
			assert.Equal(t, tt.expectedExtractedMetadata, actualExtractedMetadata)
		})
	}
}
