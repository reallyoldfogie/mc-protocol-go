#!/bin/bash
# Script to run code generation with low-memory optimizations

echo "Building generator..."
go build -mod=readonly -o generator ./cmd/generator

if [ $? -ne 0 ]; then
    echo "Failed to build generator"
    exit 1
fi

echo "Running generator with low-memory settings..."
echo ""

# Run with default config which has low-memory settings
./generator
