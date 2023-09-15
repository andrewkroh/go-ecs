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

import "testing"

func TestLookup(t *testing.T) {
	testCases := []struct {
		Field   string
		Version string
		Fail    bool
	}{
		{Field: "labels", Version: "v8.9"}, // Leading 'v' is tolerated.

		{Field: "labels", Version: "8"},
		{Field: "labels", Version: "8.9"},
		{Field: "labels", Version: "8.9.0"},

		{Field: "labels", Version: "v8.9.1", Fail: true}, // Version does not exists.

		{Field: "labels", Version: ""}, // No version should use the latest.
		{Field: "labels", Version: "v", Fail: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Version, func(t *testing.T) {
			f, err := Lookup(tc.Field, tc.Version)
			if tc.Fail {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if f == nil {
				t.Fatal("expected non-nil field, but got nil")
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
