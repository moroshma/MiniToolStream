#!/bin/bash

echo "========================================="
echo "  Stopping Client Port Forwards"
echo "========================================="
echo ""

# Kill processes on port 50051
if lsof -Pi :50051 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "Stopping Ingress port-forward (50051)..."
    kill $(lsof -t -i:50051) 2>/dev/null
    echo "✓ Ingress port-forward stopped"
else
    echo "ℹ️  No process found on port 50051"
fi

# Kill processes on port 50052
if lsof -Pi :50052 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "Stopping Egress port-forward (50052)..."
    kill $(lsof -t -i:50052) 2>/dev/null
    echo "✓ Egress port-forward stopped"
else
    echo "ℹ️  No process found on port 50052"
fi

echo ""
echo "========================================="
echo "  All port-forwards stopped"
echo "========================================="
