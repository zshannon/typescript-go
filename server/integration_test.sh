#!/bin/bash
# Integration test suite for TypeScript-Go Server
# Usage: ./integration_test.sh [base_url]
# Default: http://localhost:8080

# Don't use set -e, we handle errors manually

BASE_URL="${1:-http://localhost:8080}"
PASSED=0
FAILED=0
TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((PASSED++))
    ((TOTAL++))
}

log_fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    echo -e "  ${YELLOW}Expected${NC}: $2"
    echo -e "  ${YELLOW}Got${NC}: $3"
    ((FAILED++))
    ((TOTAL++))
}

# Test helper: check if response contains expected string
assert_contains() {
    local test_name="$1"
    local response="$2"
    local expected="$3"

    if echo "$response" | grep -q "$expected"; then
        log_pass "$test_name"
    else
        log_fail "$test_name" "contains '$expected'" "${response:0:200}..."
    fi
}

# Test helper: check if response equals expected JSON key
assert_json_key() {
    local test_name="$1"
    local response="$2"
    local key="$3"
    local expected="$4"

    local actual
    actual=$(echo "$response" | jq -r ".$key" 2>/dev/null)
    if [ "$actual" = "$expected" ]; then
        log_pass "$test_name"
    else
        log_fail "$test_name" "$key=$expected" "$key=$actual"
    fi
}

# Test helper: check if response has JSON key
assert_has_key() {
    local test_name="$1"
    local response="$2"
    local key="$3"

    if echo "$response" | jq -e ".$key" >/dev/null 2>&1; then
        log_pass "$test_name"
    else
        log_fail "$test_name" "has key '$key'" "${response:0:200}..."
    fi
}

# Test helper: check response size is greater than threshold
assert_size_gt() {
    local test_name="$1"
    local response="$2"
    local min_size="$3"

    local actual_size=${#response}
    if [ "$actual_size" -gt "$min_size" ]; then
        log_pass "$test_name (${actual_size} bytes)"
    else
        log_fail "$test_name" ">$min_size bytes" "$actual_size bytes"
    fi
}

echo "============================================"
echo "TypeScript-Go Server Integration Tests"
echo "Base URL: $BASE_URL"
echo "============================================"
echo ""

# Wait for server to be ready
echo "Waiting for server..."
for i in {1..30}; do
    if curl -s "$BASE_URL/health" >/dev/null 2>&1; then
        echo "Server is ready!"
        echo ""
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}Server not responding after 30 seconds${NC}"
        exit 1
    fi
    sleep 1
done

# ============================================
# Health Check Tests
# ============================================
echo "--- Health Check Tests ---"

response=$(curl -s "$BASE_URL/health")
assert_json_key "Health endpoint returns healthy status" "$response" "status" "healthy"
assert_has_key "Health endpoint returns version" "$response" "version"
assert_has_key "Health endpoint returns uptime" "$response" "uptime"
assert_has_key "Health endpoint returns cache_size" "$response" "cache_size"

echo ""

# ============================================
# Build Tests
# ============================================
echo "--- Build Tests ---"

# Simple build
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const hello = \"world\";", "version": "0.0.4"}')
assert_has_key "Simple build returns code" "$response" "code"
assert_contains "Simple build output contains export" "$response" "hello"

# React component build
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "import React from '\''react'\'';\nexport default () => React.createElement('\''div'\'', null, '\''Hello'\'');", "version": "0.0.4"}')
assert_has_key "React build returns code" "$response" "code"
assert_contains "React build includes React global" "$response" "_CRAYONCORE_"

# @crayonnow/core import
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "import { Text } from '\''@crayonnow/core'\'';\nexport default () => <Text>Hello</Text>;", "version": "0.0.4"}')
assert_has_key "@crayonnow/core build returns code" "$response" "code"

# Complex component with state
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "import { Flex, Text, Button } from '\''@crayonnow/core'\'';\nimport { useState } from '\''react'\'';\n\nexport default () => {\n  const [count, setCount] = useState(0);\n  return <Flex><Text>{count}</Text><Button onClick={() => setCount(count + 1)}>+</Button></Flex>;\n};", "version": "0.0.4"}')
code=$(echo "$response" | jq -r '.code' 2>/dev/null)
assert_size_gt "Complex component builds to >100KB" "$code" 100000

echo ""

# ============================================
# TypeCheck Tests
# ============================================
echo "--- TypeCheck Tests ---"

# Valid code
response=$(curl -s -X POST "$BASE_URL/typecheck" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const hello: string = \"world\";", "version": "0.0.4"}')
assert_json_key "Valid code passes type check" "$response" "pass" "true"

# Type error: number assigned to string
response=$(curl -s -X POST "$BASE_URL/typecheck" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const hello: string = 123;", "version": "0.0.4"}')
assert_has_key "Type error returns errors array" "$response" "errors"
assert_contains "Type error message is correct" "$response" "Type 'number' is not assignable to type 'string'"

# Type error with line/column info
line=$(echo "$response" | jq -r '.errors[0].line' 2>/dev/null)
column=$(echo "$response" | jq -r '.errors[0].column' 2>/dev/null)
if [ "$line" = "1" ] && [ "$column" -gt "0" ]; then
    log_pass "Type error includes line/column info (line=$line, column=$column)"
else
    log_fail "Type error includes line/column info" "line=1, column>0" "line=$line, column=$column"
fi

# Multiple type errors
response=$(curl -s -X POST "$BASE_URL/typecheck" \
    -H "Content-Type: application/json" \
    -d '{"code": "const a: string = 1;\nconst b: number = \"x\";", "version": "0.0.4"}')
error_count=$(echo "$response" | jq '.errors | length' 2>/dev/null)
if [ "$error_count" = "2" ]; then
    log_pass "Multiple type errors detected ($error_count errors)"
else
    log_fail "Multiple type errors detected" "2 errors" "$error_count errors"
fi

echo ""

# ============================================
# Build with Type Validation Tests
# ============================================
echo "--- Build with Type Validation Tests ---"

# Build with valid types
response=$(curl -s -X POST "$BASE_URL/build?validate_types=true" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const hello: string = \"world\";", "version": "0.0.4"}')
assert_has_key "Valid code builds with validation" "$response" "code"

# Build with type errors should fail
response=$(curl -s -X POST "$BASE_URL/build?validate_types=true" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const hello: string = 123;", "version": "0.0.4"}')
assert_has_key "Type error fails build with validation" "$response" "errors"
assert_contains "Type error message in build response" "$response" "Type 'number' is not assignable to type 'string'"

echo ""

# ============================================
# Edge Cases
# ============================================
echo "--- Edge Case Tests ---"

# Empty code should return error
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "", "version": "0.0.4"}')
assert_contains "Empty code returns error" "$response" "required"

# JSX syntax
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "export default () => <div className=\"test\"><span>Hello</span></div>;", "version": "0.0.4"}')
assert_has_key "JSX syntax builds successfully" "$response" "code"

# TypeScript features
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "interface User { name: string; age: number; }\nconst user: User = { name: \"John\", age: 30 };\nexport default user;", "version": "0.0.4"}')
assert_has_key "TypeScript interfaces build successfully" "$response" "code"

# Async/await
response=$(curl -s -X POST "$BASE_URL/build" \
    -H "Content-Type: application/json" \
    -d '{"code": "export const fetchData = async () => { const data = await Promise.resolve(42); return data; };", "version": "0.0.4"}')
assert_has_key "Async/await builds successfully" "$response" "code"

echo ""

# ============================================
# Summary
# ============================================
echo "============================================"
echo "Test Results"
echo "============================================"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Total:  $TOTAL"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
