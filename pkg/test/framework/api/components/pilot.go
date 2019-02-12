//  Copyright 2018 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package components

import (
	"testing"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"istio.io/istio/pkg/test/framework/api/component"
	"istio.io/istio/pkg/test/framework/api/ids"
)

// Pilot testing component
type Pilot interface {
	component.Instance
	CallDiscovery(req *xdsapi.DiscoveryRequest) (*xdsapi.DiscoveryResponse, error)
	StartDiscovery(req *xdsapi.DiscoveryRequest) error
	WatchDiscovery(duration time.Duration, accept func(*xdsapi.DiscoveryResponse) (bool, error)) error
}

// Structured config for the Pilot component
type PilotConfig struct {
	component.Configuration
	// If set then pilot takes a dependency on the referenced Galley instance
	Galley component.Requirement
}

// Create a configured Pilot component definition
func ConfigurePilot(id string, config PilotConfig) component.Requirement {
	descriptor := component.NewDescriptor(ids.Pilot, component.Variant(id))
	descriptor.Configuration = config
	if config.Galley != nil {
		descriptor.Requires = append(descriptor.Requires, config.Galley)
	}
	return descriptor
}

// GetPilot from the repository
func GetPilot(e component.Repository, t testing.TB, requirement component.Requirement) Pilot {
	return e.GetComponentOrFail(requirement, t).(Pilot)
}
