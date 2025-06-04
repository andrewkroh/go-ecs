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

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type field struct {
	AllowedValues       []allowedValue `json:"allowed_values" yaml:"allowed_values"`
	Beta                string         `json:"beta" yaml:"beta"`
	DashedName          string         `json:"dashed_name" yaml:"dashed_name"`
	Description         string         `json:"description" yaml:"description"`
	DocValues           bool           `json:"doc_values" yaml:"doc_values"`
	Example             any            `json:"example" yaml:"example"`
	ExpectedValues      []string       `json:"expected_values" yaml:"expected_values"`
	FlatName            string         `json:"flat_name" yaml:"flat_name"`
	Format              string         `json:"format" yaml:"format"`
	IgnoreAbove         int            `json:"ignore_above" yaml:"ignore_above"`
	Index               bool           `json:"index" yaml:"index"`
	InputFormat         string         `json:"input_format" yaml:"input_format"`
	Level               string         `json:"level" yaml:"level"`
	MultiFields         []multiField   `json:"multi_fields" yaml:"multi_fields"`
	Name                string         `json:"name" yaml:"name"`
	Normalize           any            `json:"normalize" yaml:"normalize"` // Only 8.5.0 has a string. All others use a []string.
	Norms               bool           `json:"norms" yaml:"norms"`
	ObjectType          string         `json:"object_type" yaml:"object_type"`
	Order               int            `json:"order" yaml:"order"`
	OriginalFieldset    string         `json:"original_fieldset" yaml:"original_fieldset"`
	OutputFormat        string         `json:"output_format" yaml:"output_format"`
	OutputPrecision     int            `json:"output_precision" yaml:"output_precision"`
	OTEL                []otel         `json:"otel" yaml:"otel"`
	Pattern             string         `json:"pattern" yaml:"pattern"`
	Pattern2            string         `json:"patther" yaml:"patther"` // Handle bug in 8.3.0.
	Required            bool           `json:"required" yaml:"required"`
	ScalingFactor       int            `json:"scaling_factor" yaml:"scaling_factor"`
	Short               string         `json:"short" yaml:"short"`
	SyntheticSourceKeep string         `json:"synthetic_source_keep" yaml:"synthetic_source_keep"` // Values are none (default), arrays, all.
	Type                string         `json:"type" yaml:"type"`
}

type allowedValue struct {
	Beta               *string  `json:"beta,omitempty" yaml:"beta,omitempty"` // Message that warns about beta features.
	Description        string   `json:"description" yaml:"description"`
	ExpectedEventTypes []string `json:"expected_event_types" yaml:"expected_event_types"`
	Name               string   `json:"name" yaml:"name"`
}

type multiField struct {
	FlatName string `json:"flat_name" yaml:"flat_name"`
	Name     string `json:"name" yaml:"name"`
	Norms    bool   `json:"norms" yaml:"norms"`
	Type     string `json:"type" yaml:"type"`
}

type ecsField struct {
	Name              string `json:"name,omitempty"`               // Flattened name.
	Description       string `json:"description,omitempty"`        // Full description.
	DataType          string `json:"data_type,omitempty"`          // Elasticsearch field data type (e.g. boolean, keyword, text).
	ValidationPattern string `json:"validation_pattern,omitempty"` // Regular expression for validating values of the field.
	Array             bool   `json:"array,omitempty"`              // Indicates if the value is an array.
}

type otel struct {
	Attribute string `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Metric    string `json:"metric,omitempty" yaml:"metric,omitempty"`
	OTLPField string `json:"otlp_field,omitempty" yaml:"otlp_field,omitempty"`
	Relation  string `json:"relation" yaml:"relation"`
	Stability string `json:"stability" yaml:"stability"`
	Note      string `json:"note,omitempty" yaml:"note,omitempty"`
}

func newECSField(f field) ecsField {
	pattern := f.pattern()
	if pattern == "" {
		if len(f.AllowedValues) > 0 {
			var values []string
			for _, v := range f.AllowedValues {
				// This is a simplified representation that drops
				// event.category to event.type constraint info.
				values = append(values, v.Name)
			}
			slices.Sort(values)
			values = slices.Compact(values)
			pattern = makePattern(values)
		} else if len(f.ExpectedValues) > 0 {
			slices.Sort(f.ExpectedValues)
			values := slices.Compact(f.ExpectedValues)
			pattern = makePattern(values)
		}
	}

	return ecsField{
		Name:              f.FlatName,
		Description:       f.Description,
		DataType:          f.Type,
		ValidationPattern: pattern,
		Array:             f.isArray(),
	}
}

func (f *field) isArray() bool {
	switch v := f.Normalize.(type) {
	case []any:
		for _, item := range v {
			switch v := item.(type) {
			case string:
				if v == "array" {
					return true
				}

				panic(fmt.Errorf("unhandled value for 'normalize' field of %q: %q", f.Name, v))
			default:
				panic(fmt.Errorf("unhandled value type for 'normalize' field value type %T in field %q", item, f.Name))
			}
		}
	case string:
		if v == "array" {
			return true
		}
		panic(fmt.Errorf("unhandled value for 'normalize' field of %q: %q", f.Name, v))
	case nil:
	default:
		panic(fmt.Errorf("unhandled value type for 'normalize' field value type %T in field %q", f.Normalize, f.Name))
	}
	return false
}

func (f *field) pattern() string {
	pattern := f.Pattern
	if pattern == "" && f.Pattern2 != "" {
		pattern = f.Pattern2
	}
	if pattern != "" {
		_, err := regexp.Compile(pattern)
		if err != nil {
			panic(fmt.Errorf("invalid 'pattern' for field %q: %w", f.FlatName, err))
		}
	}
	return pattern
}

func makePattern(allowedValues []string) string {
	return fmt.Sprintf("(?:%s)", strings.Join(allowedValues, "|"))
}
