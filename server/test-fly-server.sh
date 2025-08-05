#!/bin/bash

# Test the Fly hosted TypeScript server

echo "Testing typecheck endpoint..."
curl -X POST https://server-wild-sea-9370.fly.dev/typecheck \
   -H "Content-Type: application/json" \
   -d @test-fortune-cookie.json

echo -e "\n\n"

echo "Testing build endpoint..."
curl -X POST https://server-wild-sea-9370.fly.dev/build \
   -H "Content-Type: application/json" \
   -d @test-fortune-cookie.json