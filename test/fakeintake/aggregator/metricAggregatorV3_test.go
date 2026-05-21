// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025-present Datadog, Inc.

package aggregator

import (
	"testing"
	"time"

	"github.com/DataDog/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/dogstatsdhttp"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/DataDog/datadog-agent/test/fakeintake/api"
)

const (
	testFlagHasUnit = 0x200

	testFieldDictUnitStr protowire.Number = 25
	testFieldUnitRefs    protowire.Number = 26
)

// buildMinimalV3Payload constructs a pb.Payload containing a single gauge metric
// "test.gauge" with tag "env:test", timestamp 1000, and value 42.0.
//
// Wire encoding of the column-oriented MetricData:
//   - DictNameStr: varint(10) + "test.gauge"
//   - DictTagStr:  varint(8)  + "env:test"
//   - DictTagsets: [1, 1]          — one tagset of size 1, pointing to dictTagStr[1]
//   - Types:       [3|48] = [51]   — MetricType_Gauge | ValueType_Float64
//   - NameRefs:    [1]             — delta-encoded index → dictNameStr[1]
//   - TagsetRefs:  [1]             — delta-encoded index → dictTagsets[1]
//   - DictResourceStr: varint(4) + "host" + varint(6) + "node-a"
//   - DictResourceLen:  [1]         — one resource set with one resource
//   - DictResourceType: [1]         — type → dictResourceStr[1] ("host")
//   - DictResourceName: [2]         — name → dictResourceStr[2] ("node-a")
//   - ResourcesRefs: [1]            — resource set → dictResources[1]
//   - SourceTypeNameRefs: [0]
//   - OriginInfoRefs: [0]
//   - Intervals:   [0]
//   - NumPoints:   [1]
//   - Timestamps:  [1000]          — delta-encoded
//   - ValsFloat64: [42.0]
func buildMinimalV3Payload() *pb.Payload {
	nameStr := append([]byte{10}, []byte("test.gauge")...) // varint(10) + "test.gauge"
	tagStr := append([]byte{8}, []byte("env:test")...)     // varint(8) + "env:test"
	resourceStr := append([]byte{4}, []byte("host")...)    // varint(4) + "host"
	resourceStr = append(resourceStr, byte(6))
	resourceStr = append(resourceStr, []byte("node-a")...)

	return &pb.Payload{
		MetricData: &pb.MetricData{
			DictNameStr:        nameStr,
			DictTagStr:         tagStr,
			DictTagsets:        []int64{1, 1}, // size=1, ref=1
			DictResourceStr:    resourceStr,
			DictResourceLen:    []int64{1},
			DictResourceType:   []int64{1},
			DictResourceName:   []int64{2},
			Types:              []uint64{uint64(pb.MetricType_Gauge) | uint64(pb.ValueType_Float64)},
			NameRefs:           []int64{1},
			TagsetRefs:         []int64{1},
			ResourcesRefs:      []int64{1},
			SourceTypeNameRefs: []int64{0},
			OriginInfoRefs:     []int64{0},
			Intervals:          []uint64{0},
			NumPoints:          []uint64{1},
			Timestamps:         []int64{1000},
			ValsFloat64:        []float64{42.0},
		},
	}
}

func buildV3PayloadWithUnit(unit string) *pb.Payload {
	p := buildMinimalV3Payload()
	p.MetricData.Types[0] |= testFlagHasUnit

	unitStr := protowire.AppendVarint(nil, uint64(len(unit)))
	unitStr = append(unitStr, []byte(unit)...)
	var unknown []byte
	unknown = appendBytesField(unknown, testFieldDictUnitStr, unitStr)
	unknown = appendBytesField(unknown, testFieldUnitRefs, []byte{2})
	p.MetricData.ProtoReflect().SetUnknown(unknown)

	return p
}

func appendBytesField(dst []byte, field protowire.Number, value []byte) []byte {
	dst = protowire.AppendTag(dst, field, protowire.BytesType)
	return protowire.AppendBytes(dst, value)
}

func TestParseMetricSeriesV3_SingleGauge(t *testing.T) {
	p := buildMinimalV3Payload()

	raw, err := p.MarshalVT()
	require.NoError(t, err)

	payload := api.Payload{
		Data:        raw,
		Encoding:    encodingEmpty, // no compression
		ContentType: "application/x-protobuf",
		Timestamp:   time.Unix(999, 0),
	}

	series, err := ParseMetricSeriesV3(payload)
	require.NoError(t, err)
	require.Len(t, series, 1)

	s := series[0]
	assert.Equal(t, "test.gauge", s.Metric)
	assert.Equal(t, []string{"env:test"}, s.Tags)
	require.Len(t, s.Resources, 1)
	assert.Equal(t, "host", s.Resources[0].Type)
	assert.Equal(t, "node-a", s.Resources[0].Name)
	assert.Equal(t, pb.MetricType_Gauge, s.Type)
	assert.Empty(t, s.Unit)
	require.Len(t, s.Points, 1)
	assert.Equal(t, int64(1000), s.Points[0].Timestamp)
	assert.Equal(t, 42.0, s.Points[0].Value)
	assert.Equal(t, time.Unix(999, 0), s.GetCollectedTime())
}

func TestParseMetricSeriesV3_Unit(t *testing.T) {
	p := buildV3PayloadWithUnit("millisecond")

	raw, err := p.MarshalVT()
	require.NoError(t, err)

	payload := api.Payload{
		Data:        raw,
		Encoding:    encodingEmpty, // no compression
		ContentType: "application/x-protobuf",
		Timestamp:   time.Unix(999, 0),
	}

	series, err := ParseMetricSeriesV3(payload)
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, "millisecond", series[0].Unit)
}

func TestParseMetricSeriesV3_CompressedColumnPayload(t *testing.T) {
	p := buildMinimalV3Payload()
	raw, err := p.MarshalVT()
	require.NoError(t, err)

	compressed := buildCompressedV3Payload(t, raw)
	inflated, err := inflate(compressed, encodingZstd)
	require.NoError(t, err)
	require.Equal(t, raw, inflated)

	payload := api.Payload{
		Data:        compressed,
		Encoding:    encodingZstd,
		ContentType: "application/x-protobuf",
		Timestamp:   time.Unix(999, 0),
	}

	series, err := ParseMetricSeriesV3(payload)
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, "test.gauge", series[0].Metric)
}

func TestParseMetricSeriesV3_EmptyPayload(t *testing.T) {
	p := &pb.Payload{MetricData: nil}
	raw, err := p.MarshalVT()
	require.NoError(t, err)

	payload := api.Payload{Data: raw, Encoding: encodingEmpty, Timestamp: time.Now()}
	series, err := ParseMetricSeriesV3(payload)
	require.NoError(t, err)
	assert.Empty(t, series)
}

func buildCompressedV3Payload(t *testing.T, raw []byte) []byte {
	t.Helper()

	fieldNum, typ, n := protowire.ConsumeTag(raw)
	require.Greater(t, n, 0)
	require.Equal(t, protowire.Number(3), fieldNum)
	require.Equal(t, protowire.BytesType, typ)
	metricData, m := protowire.ConsumeBytes(raw[n:])
	require.Greater(t, m, 0)
	require.Equal(t, len(raw), n+m)

	out := zstdCompress(t, raw[:len(raw)-len(metricData)])
	for len(metricData) > 0 {
		_, fieldType, fieldHeaderLen := protowire.ConsumeTag(metricData)
		require.Greater(t, fieldHeaderLen, 0)
		require.Equal(t, protowire.BytesType, fieldType)
		fieldValue, fieldValueLen := protowire.ConsumeBytes(metricData[fieldHeaderLen:])
		require.Greater(t, fieldValueLen, 0)
		headerLen := fieldHeaderLen + fieldValueLen - len(fieldValue)
		out = append(out, zstdCompress(t, metricData[:headerLen])...)
		out = append(out, zstdCompress(t, fieldValue)...)
		metricData = metricData[headerLen+len(fieldValue):]
	}
	return out
}

func zstdCompress(t *testing.T, raw []byte) []byte {
	t.Helper()
	compressed, err := zstd.CompressLevel(nil, raw, 1)
	require.NoError(t, err)
	return compressed
}

func TestParseMetricSeriesV3_EmptyBody(t *testing.T) {
	for _, payload := range []api.Payload{
		{Data: nil, Encoding: encodingEmpty, Timestamp: time.Now()},
		{Data: []byte("{}"), Encoding: encodingEmpty, Timestamp: time.Now()},
		{Data: nil, Encoding: encodingZstd, Timestamp: time.Now()},
	} {
		series, err := ParseMetricSeriesV3(payload)
		require.NoError(t, err)
		assert.Empty(t, series)
	}
}

func TestParseMetricSeriesV3_SketchSkipped(t *testing.T) {
	// A payload with a sketch entry should produce no MetricSeriesV3 items —
	// sketches are intentionally drained and excluded on the series endpoint.
	nameStr := append([]byte{10}, []byte("test.dist1")...)
	p := &pb.Payload{
		MetricData: &pb.MetricData{
			DictNameStr:        nameStr,
			DictTagStr:         nil,
			DictTagsets:        []int64{0},
			Types:              []uint64{uint64(pb.MetricType_Sketch) | uint64(pb.ValueType_Zero)},
			NameRefs:           []int64{1},
			TagsetRefs:         []int64{0},
			ResourcesRefs:      []int64{0},
			SourceTypeNameRefs: []int64{0},
			OriginInfoRefs:     []int64{0},
			Intervals:          []uint64{0},
			NumPoints:          []uint64{1},
			Timestamps:         []int64{1000},
			// sketch with zero bins and zero-valued summary
			SketchNumBins: []uint64{0},
			ValsSint64:    []int64{0}, // cnt
		},
	}

	raw, err := p.MarshalVT()
	require.NoError(t, err)

	payload := api.Payload{Data: raw, Encoding: encodingEmpty, Timestamp: time.Now()}
	series, err := ParseMetricSeriesV3(payload)
	require.NoError(t, err)
	assert.Empty(t, series, "sketch entries must be skipped on the series endpoint")
}

func TestParseMetricSeriesV3_Aggregator(t *testing.T) {
	// End-to-end: parse through the aggregator so GetPayloadsByName works.
	p := buildMinimalV3Payload()
	raw, err := p.MarshalVT()
	require.NoError(t, err)

	agg := NewMetricAggregatorV3()
	err = agg.UnmarshallPayloads([]api.Payload{
		{Data: raw, Encoding: encodingEmpty, Timestamp: time.Now()},
	})
	require.NoError(t, err)

	byName := agg.GetPayloadsByName("test.gauge")
	require.Len(t, byName, 1)
	assert.Equal(t, "test.gauge", byName[0].Metric)
	assert.Equal(t, []string{"env:test"}, byName[0].Tags)
}
