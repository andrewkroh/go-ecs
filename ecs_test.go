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

package ecs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	testCases := []struct {
		name    string
		field   string
		version string
		wantErr error
	}{
		{
			name:    "valid field with v prefix version",
			field:   "labels",
			version: "v8.9",
			wantErr: nil,
		},
		{
			name:    "valid field with partial version",
			field:   "labels",
			version: "8",
			wantErr: nil,
		},
		{
			name:    "valid field with full version",
			field:   "labels",
			version: "8.9",
			wantErr: nil,
		},
		{
			name:    "valid field with patch version",
			field:   "labels",
			version: "8.9.0",
			wantErr: nil,
		},
		{
			name:    "valid field with empty version",
			field:   "labels",
			version: "",
			wantErr: nil,
		},
		{
			name:    "nonexistent version",
			field:   "labels",
			version: "v8.9.1",
			wantErr: ErrVersionNotFound,
		},
		{
			name:    "invalid version - only v",
			field:   "labels",
			version: "v",
			wantErr: ErrInvalidVersion,
		},
		{
			name:    "nonexistent field",
			field:   "nonexistent.field",
			version: "8.9",
			wantErr: ErrFieldNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Lookup(tc.field, tc.version)

			if tc.wantErr != nil {
				if err != tc.wantErr { //nolint:errorlint // These errors are never wrapped.
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if f == nil {
				t.Fatal("expected non-nil field, but got nil")
			}
		})
	}
}

func TestFields(t *testing.T) {
	testCases := []struct {
		name    string
		version string
		wantErr error
	}{
		{
			name:    "empty version returns latest",
			version: "",
			wantErr: nil,
		},
		{
			name:    "valid version with v prefix",
			version: "v8.9",
			wantErr: nil,
		},
		{
			name:    "valid version without prefix",
			version: "8.9",
			wantErr: nil,
		},
		{
			name:    "partial version",
			version: "8",
			wantErr: nil,
		},
		{
			name:    "invalid version - only v",
			version: "v",
			wantErr: ErrInvalidVersion,
		},
		{
			name:    "nonexistent version",
			version: "99.99.99",
			wantErr: ErrVersionNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fields, err := Fields(tc.version)

			if tc.wantErr != nil {
				if err != tc.wantErr { //nolint:errorlint // These errors are never wrapped.
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fields == nil {
				t.Fatal("expected non-nil fields map")
			}

			if len(fields) == 0 {
				t.Fatal("expected non-empty fields map")
			}

			if _, exists := fields["labels"]; !exists {
				t.Error("expected 'labels' field to exist in fields map")
			}
		})
	}
}

func BenchmarkLookup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Lookup("labels", "v8")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFields(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Fields("v8")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func ExampleLookup() {
	field, err := Lookup("host.os.name", "8.10")
	if err != nil {
		fmt.Println(err)
		return
	}

	data, _ := json.MarshalIndent(field, "", "  ")
	fmt.Printf("%s", data)
	// Output: {
	//   "name": "host.os.name",
	//   "data_type": "keyword",
	//   "array": false,
	//   "description": "Operating system name, without the version."
	// }
}

func ExampleFields() {
	fields, err := Fields("8.10")
	if err != nil {
		fmt.Println(err)
		return
	}

	var hostFields int
	for name := range fields {
		if strings.HasPrefix(name, "host.") {
			hostFields++
		}
	}

	fmt.Println("Total fields in ECS 8.10:", len(fields))
	fmt.Println("Total host fields:", hostFields)

	// Output: Total fields in ECS 8.10: 1659
	// Total host fields: 42
}
