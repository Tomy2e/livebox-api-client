package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
)

func main() {
	var (
		service = flag.String("service", "", "service")
		method  = flag.String("method", "", "method")
		params  = flag.String("params", "", "JSON-encoded params")
	)
	flag.Parse()

	client, err := livebox.NewClient(os.Getenv("ADMIN_PASSWORD"))
	if err != nil {
		log.Fatalf("failed to create livebox client: %s", err)
	}

	req, err := newRequest(*service, *method, *params)
	if err != nil {
		log.Fatalf("failed to create request: %s", err)
	}

	out := json.RawMessage{}
	if err := client.Request(context.Background(), req, &out); err != nil {
		log.Fatalf("request failed: %s", err)
	}

	fmt.Println(string(out))
}

func newRequest(service, method, params string) (*request.Request, error) {
	if service == "" {
		return nil, errors.New("-service is missing")
	}

	if method == "" {
		return nil, errors.New("-method is missing")
	}

	var parameters request.Parameters
	if params != "" {
		if err := json.Unmarshal([]byte(params), &parameters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal params: %w", err)
		}
	}

	return &request.Request{
		Service:    service,
		Method:     method,
		Parameters: parameters,
	}, nil
}
