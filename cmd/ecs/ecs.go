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

// The ecs command-line tool retrieves the definition of a field in the
// Elastic Common Schema (ECS). The tool can be used to quickly look up field
// definitions and their properties.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/andrewkroh/go-ecs"
)

var usage = `ecs [field]

ecs is a command-line tool for retrieving definitions of Elastic Common Schema
(ECS) fields. The field definition is written as JSON to stdout.

See https://www.elastic.co/guide/en/ecs/current/ecs-field-reference.html

OPTIONS:

  -h           Show this help message and exit.
  -r           ECS release version (e.g. 8.11.0 or 8.11 or 8).
               Defaults to latest version incorporated into
               github.com/andrewkroh/go-ecs at build time.
  -q           Quiet mode. No ECS definition is written to stdout.

ARGUMENTS:

  field        The name of the ECS field to retrieve the definition for.
               This argument is required.

EXAMPLES:

  ecs source.ip
    Retrieves the JSON definition of the "source.ip" ECS field. 

EXIT STATUS:

   0       Successful completion. Field is defined in ECS.
   1       Field not defined.
   2       Usage/argument problem.
`

var (
	ecsVersion = flag.String("r", "", "ECS release version")
	quiet      = flag.Bool("q", false, "Quiet mode")
)

func main() {
	// Flag handling.
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
	}
	if len(flag.Args()) > 1 {
		fmt.Fprintln(os.Stderr, "Only one field name may be specified.")
		os.Exit(2)
	}

	// ECS lookup.
	field, err := ecs.Lookup(flag.Arg(0), *ecsVersion)
	if err != nil {
		if !*quiet {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	if !*quiet {
		// Dump as pretty JSON.
		data, err := json.MarshalIndent(field, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("%s\n", data)
	}
}
