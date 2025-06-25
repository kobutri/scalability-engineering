#!/bin/bash

echo "🚀 Starting development environment..."
echo "This will start the Go server with hot reloading enabled."
echo "Make changes to your Go files and they will automatically reload!"
echo ""

# Check if required images exist
echo "🔍 Checking for required Docker images..."
if ! docker images scalability-engineering_client:latest -q | grep -q .; then
    echo "⚠️  Client image not found. Building images..."
    ./build.sh
else
    echo "✅ Required images found."
fi

echo ""
echo "🚀 Starting development environment..."

# Start development environment
docker-compose -f docker-compose.dev.yml up --build

echo ""
echo "Development environment stopped." 