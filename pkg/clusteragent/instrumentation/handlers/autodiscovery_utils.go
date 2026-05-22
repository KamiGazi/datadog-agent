// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build kubeapiserver

package handlers

import (
	"sync"

	datadoghq "github.com/DataDog/datadog-operator/api/datadoghq/v1alpha1"

	"github.com/DataDog/datadog-agent/comp/core/autodiscovery/integration"
)

// serviceTemplateEntry links a DDI CR key to a target service and its check templates.
type serviceTemplateEntry struct {
	serviceNamespace string
	serviceName      string
	templates        []integration.Config
}

// ServiceCheckTemplateStore holds check templates for Service-targeted DDI CRs.
// The handler writes templates here; a separate AD config provider reads them and
// combines with EndpointSlice data to produce per-endpoint configs.
type ServiceCheckTemplateStore struct {
	mu sync.RWMutex
	// entries maps DDI CR key (namespace/name) to the target service and templates.
	entries map[string]serviceTemplateEntry
	// onChange is called when templates are added or removed.
	onChange func()
}

// NewServiceCheckTemplateStore creates a new ServiceCheckTemplateStore.
func NewServiceCheckTemplateStore() *ServiceCheckTemplateStore {
	return &ServiceCheckTemplateStore{
		entries: make(map[string]serviceTemplateEntry),
	}
}

// SetOnChange registers a callback invoked when the template set changes.
func (s *ServiceCheckTemplateStore) SetOnChange(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// writeTemplates stores templates keyed by DDI CR,
// associating them with the target service from the CR's TargetRef.
func (s *ServiceCheckTemplateStore) writeTemplates(key string, cr *datadoghq.DatadogInstrumentation, configs []integration.Config) {
	s.mu.Lock()
	if len(configs) == 0 {
		delete(s.entries, key)
	} else {
		s.entries[key] = serviceTemplateEntry{
			serviceNamespace: cr.Namespace,
			serviceName:      cr.Spec.TargetRef.Name,
			templates:        configs,
		}
	}
	onChange := s.onChange
	s.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

// deleteTemplates removes templates keyed by DDI CR.
func (s *ServiceCheckTemplateStore) deleteTemplates(key string) {
	s.mu.Lock()
	delete(s.entries, key)
	onChange := s.onChange
	s.mu.Unlock()
	if onChange != nil {
		onChange()
	}
}

// HasService reports whether any templates target the given service.
func (s *ServiceCheckTemplateStore) HasService(namespace, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.entries {
		if entry.serviceNamespace == namespace && entry.serviceName == name {
			return true
		}
	}
	return false
}

// AllTemplatesByService returns all templates grouped by "namespace/name" service key
// in a single pass. This avoids repeated lock acquisitions per service.
func (s *ServiceCheckTemplateStore) AllTemplatesByService() map[string][]integration.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]integration.Config)
	for _, entry := range s.entries {
		key := entry.serviceNamespace + "/" + entry.serviceName
		out[key] = append(out[key], entry.templates...)
	}
	return out
}

// CheckStore stores integration.Config entries keyed by DatadogInstrumentation CR.
type CheckStore struct {
	mu      sync.RWMutex
	configs map[string][]integration.Config
}

// NewCheckStore creates a new CheckStore.
func NewCheckStore() *CheckStore {
	return &CheckStore{
		configs: make(map[string][]integration.Config),
	}
}

// ListConfigs returns a snapshot of all stored integration.Config entries.
func (c *CheckStore) ListConfigs() []integration.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []integration.Config
	for _, cfgs := range c.configs {
		out = append(out, cfgs...)
	}
	return out
}

func (c *CheckStore) writeConfigs(key string, configs []integration.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(configs) == 0 {
		delete(c.configs, key)
		return
	}
	c.configs[key] = configs
}

func (c *CheckStore) deleteConfigs(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.configs, key)
}

func isService(cr *datadoghq.DatadogInstrumentation) bool {
	return cr.Spec.TargetRef.Kind == "Service"
}
