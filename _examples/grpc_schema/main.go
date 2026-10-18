// Command grpc_schema fetches the protobuf schema of the Synthient gRPC API
// through server reflection and prints its services and files.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/grpc_schema
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/synthient/synthient-go/grpcschema"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema, err := grpcschema.Fetch(ctx, os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		explanation := grpcschema.Explain(err)
		if explanation != "" {
			return fmt.Errorf("%s %w", explanation, err)
		}
		return err
	}

	fmt.Println("endpoint:", schema.Endpoint)
	for _, symbol := range schema.Symbols {
		fmt.Println("service:", symbol)
	}
	for _, file := range schema.DescriptorSet.GetFile() {
		fmt.Printf("file: %s (package %s)\n", file.GetName(), file.GetPackage())
	}
	return nil
}
