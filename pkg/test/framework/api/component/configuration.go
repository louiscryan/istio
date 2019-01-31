//  Copyright 2019 Istio Authors
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

package component

import "fmt"

var (
	_ Requirement = &RequirementWrapper{}
)

// Configuration is a marker interface for configuration objects that components take.
type Configuration fmt.Stringer

// RequirementWrapper wraps a requirement and adds a name and a configuration.
type RequirementWrapper struct {
	Requirement Requirement
	Name        string
	Config      Configuration
}

// String implements fmt.Stringer
func (c RequirementWrapper) String() string {
	return fmt.Sprint("{name: ", c.Name, ", requirement: ", c.Requirement, ", config: ", c.Config, "}")
}

// NameRequirement wraps a requirement with a name.
func NameRequirement(req Requirement, name string) Requirement {
	// If this is already a wrapped requirement, just set the name.
	if c, ok := req.(*RequirementWrapper); ok {
		c.Name = name
		return c
	}
	// Otherwise wrap the requirement in a RequirementWrapper.
	return &RequirementWrapper{req, name, nil}
}

// ConfigureRequirement wraps a requirement with a configuration.
func ConfigureRequirement(req Requirement, config Configuration) Requirement {
	// If this is already a wrapped requirement, just set the config.
	if c, ok := req.(*RequirementWrapper); ok {
		c.Config = config
		return c
	}
	// Otherwise wrap the requirement in a RequirementWrapper.
	return &RequirementWrapper{req, "", config}
}
