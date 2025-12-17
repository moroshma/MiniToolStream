#!/bin/bash

# generate-test-files.sh
# Generates test files for benchmarking

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="$SCRIPT_DIR/../../data/test-files"

mkdir -p "$DATA_DIR"

echo "Generating test files..."
echo

# Generate 10KB file
echo "Generating 10KB test file..."
dd if=/dev/urandom of="$DATA_DIR/test-10kb.bin" bs=1024 count=10 2>/dev/null
echo "✓ Created: $DATA_DIR/test-10kb.bin"

# Generate 1MB file
echo "Generating 1MB test file..."
dd if=/dev/urandom of="$DATA_DIR/test-1mb.bin" bs=1024 count=1024 2>/dev/null
echo "✓ Created: $DATA_DIR/test-1mb.bin"

# Generate 10MB file
echo "Generating 10MB test file..."
dd if=/dev/urandom of="$DATA_DIR/test-10mb.bin" bs=1048576 count=10 2>/dev/null
echo "✓ Created: $DATA_DIR/test-10mb.bin"

# Generate 100MB file
echo "Generating 100MB test file..."
dd if=/dev/urandom of="$DATA_DIR/test-100mb.bin" bs=1048576 count=100 2>/dev/null
echo "✓ Created: $DATA_DIR/test-100mb.bin"

# Generate 1GB file (optional, takes time)
read -p "Generate 1GB test file? This will take some time (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Generating 1GB test file... (this may take a few minutes)"
    dd if=/dev/urandom of="$DATA_DIR/test-1gb.bin" bs=1048576 count=1024 2>/dev/null
    echo "✓ Created: $DATA_DIR/test-1gb.bin"
fi

echo
echo "Test files generated in: $DATA_DIR"
ls -lh "$DATA_DIR"
