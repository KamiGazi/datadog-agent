// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package aggregator

import (
	"bytes"
	"fmt"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/dogstatsdhttp"

	"github.com/DataDog/datadog-agent/test/fakeintake/aggregator/internal/reader"
	"github.com/DataDog/datadog-agent/test/fakeintake/api"
)

// MetricSeriesV3 represents a single time series decoded from a V3 metrics intake payload
// (/api/intake/metrics/v3/series). The V3 format is a column-oriented protobuf encoding
// described in pkg/proto/datadog/dogstatsdhttp/payload.proto.
type MetricSeriesV3 struct {
	Metric        string
	Tags          []string
	Resources     []Resource
	Type          pb.MetricType
	Unit          string
	Points        []V3Point
	collectedTime time.Time
}

// Resource holds a resource name and type.
type Resource struct {
	Name string
	Type string
}

// V3Point is a (timestamp, value) data point from a V3 series payload.
type V3Point struct {
	Timestamp int64
	Value     float64
}

func (m *MetricSeriesV3) name() string {
	return m.Metric
}

// GetTags returns the tags attached to this series.
func (m *MetricSeriesV3) GetTags() []string {
	return m.Tags
}

// GetCollectedTime returns when fakeintake received the payload.
func (m *MetricSeriesV3) GetCollectedTime() time.Time {
	return m.collectedTime
}

// ParseMetricSeriesV3 decodes a /api/intake/metrics/v3/series payload (compressed protobuf)
// into individual MetricSeriesV3 entries, one per (metric name, tagset) pair.
// Sketch entries embedded in the same payload are skipped; they are handled separately.
func ParseMetricSeriesV3(payload api.Payload) ([]*MetricSeriesV3, error) {
	if len(payload.Data) == 0 || bytes.Equal(payload.Data, []byte("{}")) {
		return []*MetricSeriesV3{}, nil
	}

	inflated, err := inflate(payload.Data, payload.Encoding)
	if err != nil {
		return nil, fmt.Errorf("v3 payload inflate: %w", err)
	}
	if len(inflated) == 0 || bytes.Equal(inflated, []byte("{}")) {
		return []*MetricSeriesV3{}, nil
	}

	var p pb.Payload
	if err := p.UnmarshalVT(inflated); err != nil {
		return nil, fmt.Errorf("v3 payload unmarshal: %w", err)
	}

	r := reader.NewMetricDataReader(p.MetricData)
	if err := r.Initialize(); err != nil {
		return nil, fmt.Errorf("v3 reader init: %w", err)
	}

	var series []*MetricSeriesV3
	for r.HaveMoreMetrics() {
		if err := r.NextMetric(); err != nil {
			return nil, fmt.Errorf("v3 next metric: %w", err)
		}

		metricType := r.Type()
		if metricType == pb.MetricType_Sketch {
			// Drain the sketch's points so the reader advances correctly,
			// but don't produce a series entry — sketches have their own aggregator.
			for r.HaveMorePoints() {
				if err := r.NextPoint(); err != nil {
					return nil, fmt.Errorf("v3 next sketch point: %w", err)
				}
			}
			continue
		}

		s := &MetricSeriesV3{
			Metric:        r.Name(),
			Tags:          append([]string{}, r.Tags()...),
			Resources:     cloneResources(r.Resources()),
			Type:          metricType,
			Unit:          r.Unit(),
			collectedTime: payload.Timestamp,
		}

		for r.HaveMorePoints() {
			if err := r.NextPoint(); err != nil {
				return nil, fmt.Errorf("v3 next point: %w", err)
			}
			s.Points = append(s.Points, V3Point{
				Timestamp: r.Timestamp(),
				Value:     r.Value(),
			})
		}

		series = append(series, s)
	}

	return series, nil
}

func cloneResources(resources []*reader.Resource) []Resource {
	if len(resources) == 0 {
		return nil
	}
	cloned := make([]Resource, 0, len(resources))
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		cloned = append(cloned, Resource{
			Name: resource.Name,
			Type: resource.Type,
		})
	}
	return cloned
}

// MetricAggregatorV3 stores V3 metric series payloads received on /api/intake/metrics/v3/series.
type MetricAggregatorV3 struct {
	Aggregator[*MetricSeriesV3]
}

// NewMetricAggregatorV3 returns a new MetricAggregatorV3.
func NewMetricAggregatorV3() MetricAggregatorV3 {
	return MetricAggregatorV3{
		Aggregator: newAggregator(ParseMetricSeriesV3),
	}
}
