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

// Package ecs go-ecs is a library for querying Elastic Common Schema (ECS)
// fields by name to obtain the fields definition (e.g. Elasticsearch field data
// type, description, etc.). The library includes data from tagged released of
// https://github.com/elastic/ecs.
package ecs

import (
	"errors"
	"strings"

	"github.com/andrewkroh/go-ecs/internal/version"
)

// Error types.
var (
	ErrInvalidVersion  = errors.New("invalid version")
	ErrVersionNotFound = errors.New("version not found")
	ErrFieldNotFound   = errors.New("field not found")
)

// Field represents an ECS field.
type Field = version.Field

// Lookup an ECS field definition. If ecsVersion is empty then the latest version
// is used. You may specify a partial version specifier (e.g. '8', '8.1'), and
// the latest matching version will be searched. The returned Field should not be
// modified.
func Lookup(fieldName, ecsVersion string) (*Field, error) {
	// Normalize the version.
	semVer := strings.TrimPrefix(ecsVersion, "v")
	if ecsVersion != semVer && semVer == "" {
		return nil, ErrInvalidVersion
	}

	// Find the specified version of the ECS fields.
	var fields map[string]*Field
	if semVer == "" {
		fields = version.Latest
	} else {
		var found bool
		fields, found = version.Index[semVer]
		if !found {
			return nil, ErrVersionNotFound
		}
	}

	// Lookup the field by name.
	if f, found := fields[fieldName]; found {
		return f, nil
	}

	return nil, ErrFieldNotFound
}
