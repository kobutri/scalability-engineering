#!/bin/bash

echo "🚀 Starting development environment..."
echo "This will start the Go server with hot reloading enabled."
echo "Make changes to your Go files and they will automatically reload!"
echo ""

# Start development environment
docker-compose -f docker-compose.dev.yml up --build

echo ""
echo "Development environment stopped." 