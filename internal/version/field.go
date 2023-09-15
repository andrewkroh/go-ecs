// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package version

// Field represents an ECS field.
type Field struct {
	// Flattened field name.
	Name string `json:"name" yaml:"name"`

	// Elasticsearch field data type (e.g. keyword, match_only_text).
	DataType string `json:"data_type" yaml:"data_type"`

	// Indicates if the value type must be an array.
	Array bool `json:"array" yaml:"array"`

	// Regular expression pattern that can be used to validate the value.
	Pattern string `json:"pattern" yaml:"pattern"`

	// Short description of the field.
	Description string `json:"description" yaml:"description"`
}
