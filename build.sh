#!/bin/bash

set -e  # Exit on any error

echo "🔨 Building scalability-engineering Docker images..."
echo ""

# Function to build and tag an image
build_and_tag() {
    local service=$1
    local context=$2
    
    echo "📦 Building $service service..."
    docker-compose build $service
    
    # Get the image name that docker-compose created
    local compose_image="scalability-engineering-$service"
    local underscore_image="scalability-engineering_$service"
    
    # Tag with underscore version for compatibility
    echo "🏷️  Tagging $compose_image as $underscore_image..."
    docker tag "$compose_image:latest" "$underscore_image:latest"
    
    echo "✅ $service service built and tagged successfully"
    echo ""
}

# Build bootstrap service
build_and_tag "bootstrap" "./bootstrap"

# Build client service  
build_and_tag "client" "./client"

echo "🎉 All images built successfully!"
echo ""
echo "Available images:"
docker images | grep scalability-engineering | sort

echo ""
echo "✨ Ready to start services with: ./dev.sh" 