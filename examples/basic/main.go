package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/phenoml/phenostore-sdk-go/phenostore"
)

func main() {
	client, err := phenostore.NewClient(
		os.Getenv("PHENOSTORE_URL"),
		os.Getenv("PHENOSTORE_CLIENT_ID"),
		os.Getenv("PHENOSTORE_CLIENT_SECRET"),
		"my-tenant",
		"my-store",
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Read a Patient resource
	patient, err := client.ReadResource(ctx, "Patient", "pat-123")
	if phenostore.IsNotFound(err) {
		fmt.Println("Patient not found")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Patient:", string(patient))

	// Create a new Observation
	obs := json.RawMessage(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"text": "Blood pressure"},
		"subject": {"reference": "Patient/pat-123"}
	}`)
	created, err := client.CreateResource(ctx, "Observation", obs, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Created:", string(created))

	// Search for Conditions
	bundle, err := client.SearchResources(ctx, "Condition", nil)
	if err != nil {
		log.Fatal(err)
	}
	if bundle.Entry != nil {
		fmt.Printf("Found %d conditions\n", len(*bundle.Entry))
	}

	// For operations not in the convenience layer, use Inner() with Tenant()/Store():
	// resp, err := client.Inner().PatchResourceWithResponse(ctx, client.Tenant(), client.Store(), "Patient", "pat-123", nil, patchBody)
}
