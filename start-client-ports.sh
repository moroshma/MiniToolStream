#!/bin/bash

echo "========================================="
echo "  MiniToolStream Client Ports"
echo "========================================="
echo ""
echo "Starting port-forwards for local clients..."
echo ""

# Check if ports are already in use
if lsof -Pi :50051 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "⚠️  Port 50051 (Ingress) is already in use"
    echo "   Killing existing process..."
    kill $(lsof -t -i:50051) 2>/dev/null
    sleep 1
fi

if lsof -Pi :50052 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "⚠️  Port 50052 (Egress) is already in use"
    echo "   Killing existing process..."
    kill $(lsof -t -i:50052) 2>/dev/null
    sleep 1
fi

# Start port-forwards
echo "Starting Ingress port-forward (50051)..."
kubectl port-forward -n minitoolstream svc/minitoolstream-ingress-service 50051:50051 > /tmp/ingress-pf.log 2>&1 &
INGRESS_PID=$!

echo "Starting Egress port-forward (50052)..."
kubectl port-forward -n minitoolstream svc/minitoolstream-egress-service 50052:50052 > /tmp/egress-pf.log 2>&1 &
EGRESS_PID=$!

# Wait for ports to be ready
sleep 2

# Verify connectivity
echo ""
echo "Verifying connectivity..."
if nc -zv localhost 50051 2>&1 | grep -q succeeded ; then
    echo "✓ Ingress port 50051 is accessible"
else
    echo "✗ Ingress port 50051 is NOT accessible"
fi

if nc -zv localhost 50052 2>&1 | grep -q succeeded ; then
    echo "✓ Egress port 50052 is accessible"
else
    echo "✗ Egress port 50052 is NOT accessible"
fi

echo ""
echo "========================================="
echo "  Ports are ready for clients"
echo "========================================="
echo ""
echo "Publisher (Ingress):  localhost:50051"
echo "Subscriber (Egress):  localhost:50052"
echo ""
echo "Process IDs:"
echo "  Ingress PID:  $INGRESS_PID"
echo "  Egress PID:   $EGRESS_PID"
echo ""
echo "To stop port-forwards:"
echo "  kill $INGRESS_PID $EGRESS_PID"
echo ""
echo "Or use: ./stop-client-ports.sh"
echo ""
echo "Logs available at:"
echo "  /tmp/ingress-pf.log"
echo "  /tmp/egress-pf.log"
echo "========================================="
