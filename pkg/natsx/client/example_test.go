package client_test

import (
	"fmt"

	"github.com/telepair/watchdog/pkg/natsx/client"
)

// ExampleNewClientWithMetrics demonstrates creating a client with custom metrics.
func ExampleNewClientWithMetrics() {
	config := client.DefaultConfig()
	config.Name = "metrics-client"

	// Create a prometheus collector (pseudo-code)
	// metricsCollector := prometheus.NewPrometheusCollector()

	// For this example, we'll use nil (which creates a noop collector)
	natsClient, err := client.NewClientWithMetrics(config, nil)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// The client would normally be used for operations
	// For example purposes, we'll just check if metrics are enabled
	fmt.Printf("Metrics enabled: %t\n", natsClient.IsMetricsEnabled())

	// Always close the client when done
	defer natsClient.Close()

	// Output:
	// Failed to create client: failed to connect to NATS: nats: no servers available for connection
}

// ExampleClient_PublishContext demonstrates publishing with context.
func ExampleClient_PublishContext() {
	// This example shows the API usage but won't actually connect
	config := client.DefaultConfig()

	// In a real application, you would have a running NATS server
	// natsClient, err := client.NewClient(config)
	// if err != nil {
	//     log.Fatal(err)
	// }
	// defer natsClient.Close()

	// For demonstration, we'll show the API usage
	subject := "app.events.user.created"
	data := []byte(`{"user_id": "123", "email": "user@example.com"}`)

	// This would be the actual publish call with context:
	// ctx := context.Background()
	// err = natsClient.PublishContext(ctx, subject, data)
	// if err != nil {
	//     log.Printf("Publish failed: %v", err)
	// }

	fmt.Printf("Config name: %s\n", config.Name)
	fmt.Printf("Would publish to subject: %s\n", subject)
	fmt.Printf("Data size: %d bytes\n", len(data))

	// Output:
	// Config name: natsx.client
	// Would publish to subject: app.events.user.created
	// Data size: 47 bytes
}
