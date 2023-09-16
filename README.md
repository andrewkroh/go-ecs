# go-ecs

go-ecs is a library for querying [ECS][ecs] fields by name to obtain the fields
definition (e.g. Elasticsearch field data type, description, etc).

The library includes data from tagged released of [elastic/ecs][ecs_repo].

[ecs]: https://www.elastic.co/guide/en/ecs/current/index.html
[ecs_repo]: https://github.com/elastic/ecs

## Install

`go get github.com/andrewkroh/go-ecs@main`

## Usage

```go
package main

import (
	"fmt"

	"github.com/andrewkroh/go-ecs"
)

func main() {
	field, err := ecs.Lookup("host.os.name", "8.10")
	if err != nil {
		return err
	}

	fmt.Println("data_type", field.DataType)
	fmt.Println("is array", field.Array)
	fmt.Println("pattern", field.Pattern)
	fmt.Println("description", field.Description)
}
```