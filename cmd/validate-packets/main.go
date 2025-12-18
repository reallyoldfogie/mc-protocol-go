package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/reallyoldfogie/mc-protocol-go/internal/packetlogtest"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <packet_log.jsonl>\n", os.Args[0])
		os.Exit(1)
	}

	logFile := os.Args[1]

	// Create file source
	src, err := packetlogtest.NewFileSource(logFile)
	if err != nil {
		log.Fatalf("Failed to create file source: %v", err)
	}
	defer src.Close()

	// Run round-trip validation
	ctx := context.Background()
	opts := packetlogtest.RoundTripOptions{
		Versions:         []string{"1.21.5"},
		StopOnFirstError: false, // Process all packets to see all errors
	}

	summary, err := packetlogtest.RunRoundTripChecks(ctx, src, opts)
	if err != nil {
		log.Fatalf("RunRoundTripChecks failed: %v", err)
	}

	// Print summary
	fmt.Printf("=== Packet Validation Summary ===\n")
	fmt.Printf("Total:     %d\n", summary.Total)
	fmt.Printf("Succeeded: %d\n", summary.Succeeded)
	fmt.Printf("Failed:    %d\n", summary.Failed)
	fmt.Printf("Skipped:   %d\n", summary.Skipped)
	fmt.Printf("\n")

	if len(summary.ErrorSummaries) > 0 {
		fmt.Printf("=== Errors (%d) ===\n", len(summary.ErrorSummaries))
		for i, err := range summary.ErrorSummaries {
			fmt.Printf("%d. Packet %s (ID %d) - Stage: %s\n   Error: %s\n\n",
				i+1, err.Name, err.ID, err.Stage, err.Error)
		}
	}

	if summary.Failed > 0 {
		os.Exit(1)
	}
}
