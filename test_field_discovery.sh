#!/bin/bash
# Test script for field discovery implementation

set -e

echo "=== Testing Field Discovery Implementation ==="
echo

# Build generator
echo "Building generator..."
go build -o generator ./cmd/generator
echo "✓ Generator built successfully"
echo

# Run generator with minimal config (only if data already exists)
if [ -d "data/1.21.6" ] && [ -d "data/1.21.1" ]; then
    echo "Data directories exist, running generator with field discovery..."
    ./generator -config configs/test_field_discovery.yaml 2>&1 | grep -A 5 "Field Discovery" || true
else
    echo "Skipping full generator run - data directories don't exist"
    echo "To test fully, run: ./generator -config configs/test_field_discovery.yaml"
fi
echo

# Check if models/field_accessors.go was generated
if [ -f "models/field_accessors.go" ]; then
    echo "✓ models/field_accessors.go generated"
    echo
    echo "First 50 lines of generated file:"
    head -n 50 models/field_accessors.go
    echo
    echo "Total interfaces generated:"
    grep -c "Getter interface {" models/field_accessors.go || echo "0"
    echo
else
    echo "✗ models/field_accessors.go not found"
    echo "Run the generator first with existing data"
fi

# Verify it compiles
echo "Verifying generated code compiles..."
if go build ./models/...; then
    echo "✓ Generated code compiles successfully"
else
    echo "✗ Generated code has compilation errors"
    exit 1
fi

echo
echo "=== Field Discovery Test Complete ==="
