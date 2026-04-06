#!/bin/bash

# Production endpoint
PROD_URL="https://server-wild-sea-9370.fly.dev"
RUNS=50
WARMUP=5

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== TypeScript-Go Production Benchmarks ===${NC}"
echo -e "Endpoint: ${PROD_URL}"
echo -e "Runs per test: ${RUNS}"
echo -e "Warmup runs: ${WARMUP}\n"

# Create test payloads
echo -e "${YELLOW}Creating test payloads...${NC}"

# Ensure fixtures directory exists
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
mkdir -p "$FIXTURES_DIR"

# Simple React Component
cat > "$FIXTURES_DIR/bench-simple.json" << 'EOF'
{
  "code": "import React from 'react';\\nexport default () => React.createElement('div', null, 'Hello');",
  "version": "0.0.4"
}
EOF

# @crayonnow/core Import
cat > "$FIXTURES_DIR/bench-core.json" << 'EOF'
{
  "code": "import { Text } from '@crayonnow/core';\\nexport default () => <Text>Hello</Text>;",
  "version": "0.0.4"
}
EOF

# Complex Component (Meditation Timer)
cat > "$FIXTURES_DIR/bench-meditation.json" << 'EOF'
{
  "code": "import { Flex, Text, Button, Picker } from '@crayonnow/core';\\nimport { useState, useEffect } from 'react';\\n\\nexport default () => {\\n  const [duration, setDuration] = useState('5');\\n  const [seconds, setSeconds] = useState(0);\\n  const [running, setRunning] = useState(false);\\n\\n  const handleStartPause = () => {\\n    if (running) {\\n      setRunning(false);\\n    } else {\\n      if (seconds === 0) {\\n        setSeconds(parseInt(duration, 10) * 60);\\n      }\\n      setRunning(true);\\n    }\\n  };\\n\\n  const handleReset = () => {\\n    setRunning(false);\\n    setSeconds(parseInt(duration, 10) * 60);\\n  };\\n\\n  useEffect(() => {\\n    if (running && seconds > 0) {\\n      const timer = setTimeout(() => setSeconds(seconds - 1), 1000);\\n      return () => clearTimeout(timer);\\n    }\\n    if (running && seconds === 0) {\\n      setRunning(false);\\n    }\\n  }, [running, seconds]);\\n\\n  const minutesDisplay = String(Math.floor(seconds / 60)).padStart(2, '0');\\n  const secondsDisplay = String(seconds % 60).padStart(2, '0');\\n\\n  return (\\n    <Flex style={{ alignItems: 'stretch', minHeight: '100vh', background: '#e0f7fa', padding: '20px', rowGap: '16px' }}>\\n      <Text style={{ fontSize: '24px', fontWeight: '600', textAlign: 'center', color: '#006064' }}>\\n        Meditation Timer\\n      </Text>\\n      <Picker\\n        value={duration}\\n        onChange={(val) => {\\n          setDuration(val);\\n          setSeconds(parseInt(val, 10) * 60);\\n          setRunning(false);\\n        }}\\n        style={{ background: 'white', borderRadius: '8px', padding: '12px' }}\\n      >\\n        <Text value=\\\"5\\\">5 Minutes</Text>\\n        <Text value=\\\"10\\\">10 Minutes</Text>\\n        <Text value=\\\"15\\\">15 Minutes</Text>\\n        <Text value=\\\"20\\\">20 Minutes</Text>\\n      </Picker>\\n      <Text style={{ fontSize: '48px', fontWeight: 'bold', textAlign: 'center', color: '#004d40' }}>\\n        {minutesDisplay}:{secondsDisplay}\\n      </Text>\\n      <Button onClick={handleStartPause} style={{ background: running ? '#ff9800' : '#34C759', borderRadius: '8px', padding: '12px' }}>\\n        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>{running ? 'Pause' : 'Start'}</Text>\\n      </Button>\\n      <Button onClick={handleReset} style={{ background: '#f44336', borderRadius: '8px', padding: '12px' }}>\\n        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>Reset</Text>\\n      </Button>\\n    </Flex>\\n  );\\n};",
  "version": "0.0.4"
}
EOF

# Type Error Test
cat > "$FIXTURES_DIR/bench-type-error.json" << 'EOF'
{
  "code": "export const hello: string = 123",
  "version": "0.0.4"
}
EOF

# Valid TypeScript for type checking
cat > "$FIXTURES_DIR/bench-type-valid.json" << 'EOF'
{
  "code": "export const hello: string = 'world'; export const count: number = 42;",
  "version": "0.0.4"
}
EOF

# V3 fixtures: package.json with deps and esbuild config
cat > "$FIXTURES_DIR/bench-v3-package.json" << 'EOF'
{"main": "/index.tsx", "dependencies": {"@crayonnow/core": "1.0.0", "react": "18.0.0"}, "esbuild": {"bundle": true}}
EOF

# V3 fixtures: tsconfig.json
cat > "$FIXTURES_DIR/bench-v3-tsconfig.json" << 'EOF'
{"compilerOptions": {"lib": ["ES2022", "DOM"], "jsx": "react-jsx", "jsxImportSource": "@crayonnow/core", "module": "commonjs", "moduleResolution": "bundler", "skipLibCheck": true, "strict": true, "target": "es2022"}}
EOF

# V3 fixtures: simple component source
cat > "$FIXTURES_DIR/bench-v3-simple.tsx" << 'EOF'
import { Text } from '@crayonnow/core';

interface GreetingProps {
  name: string;
}

export default ({ name }: GreetingProps) => (
  <Text style={{ fontSize: '18px', fontWeight: '600' }}>
    Hello, {name}!
  </Text>
);
EOF

# V3 fixtures: medium component source (meditation timer)
cat > "$FIXTURES_DIR/bench-v3-medium.tsx" << 'MEDEOF'
import { Flex, Text, Button, Picker } from '@crayonnow/core';
import { useState, useEffect } from 'react';

export default () => {
  const [duration, setDuration] = useState('5');
  const [seconds, setSeconds] = useState(0);
  const [running, setRunning] = useState(false);

  const handleStartPause = () => {
    if (running) {
      setRunning(false);
    } else {
      if (seconds === 0) {
        setSeconds(parseInt(duration, 10) * 60);
      }
      setRunning(true);
    }
  };

  const handleReset = () => {
    setRunning(false);
    setSeconds(parseInt(duration, 10) * 60);
  };

  useEffect(() => {
    if (running && seconds > 0) {
      const timer = setTimeout(() => setSeconds(seconds - 1), 1000);
      return () => clearTimeout(timer);
    }
    if (running && seconds === 0) {
      setRunning(false);
    }
  }, [running, seconds]);

  const minutesDisplay = String(Math.floor(seconds / 60)).padStart(2, '0');
  const secondsDisplay = String(seconds % 60).padStart(2, '0');

  return (
    <Flex style={{ alignItems: 'stretch', minHeight: '100vh', background: '#e0f7fa', padding: '20px', rowGap: '16px' }}>
      <Text style={{ fontSize: '24px', fontWeight: '600', textAlign: 'center', color: '#006064' }}>
        Meditation Timer
      </Text>
      <Picker
        value={duration}
        onChange={(val) => {
          setDuration(val);
          setSeconds(parseInt(val, 10) * 60);
          setRunning(false);
        }}
        style={{ background: 'white', borderRadius: '8px', padding: '12px' }}
      >
        <Text value="5">5 Minutes</Text>
        <Text value="10">10 Minutes</Text>
        <Text value="15">15 Minutes</Text>
        <Text value="20">20 Minutes</Text>
      </Picker>
      <Text style={{ fontSize: '48px', fontWeight: 'bold', textAlign: 'center', color: '#004d40' }}>
        {minutesDisplay}:{secondsDisplay}
      </Text>
      <Button onClick={handleStartPause} style={{ background: running ? '#ff9800' : '#34C759', borderRadius: '8px', padding: '12px' }}>
        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>{running ? 'Pause' : 'Start'}</Text>
      </Button>
      <Button onClick={handleReset} style={{ background: '#f44336', borderRadius: '8px', padding: '12px' }}>
        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>Reset</Text>
      </Button>
    </Flex>
  );
};
MEDEOF

# V3 fixtures: dummy bun.lock
cat > "$FIXTURES_DIR/bench-v3-bun.lock" << 'EOF'
bench-lock-content
EOF

# V3 fixtures: trivial source
cat > "$FIXTURES_DIR/bench-v3-trivial.tsx" << 'EOF'
export const x: number = 1;
EOF

echo -e "${GREEN}Test payloads created${NC}\n"

# Function to run benchmark
run_benchmark() {
    local test_name="$1"
    local command="$2"
    echo -e "${BLUE}Benchmark: ${test_name}${NC}"
    hyperfine \
        --warmup "$WARMUP" \
        --runs "$RUNS" \
        --export-json "/tmp/bench-${test_name// /-}.json" \
        --export-markdown "/tmp/bench-${test_name// /-}.md" \
        "$command"
    echo ""
}

# Run benchmarks
echo -e "${YELLOW}Starting benchmarks...${NC}\n"

# 1. Health Check
run_benchmark "Health Check" \
    "curl -s ${PROD_URL}/health > /dev/null"

# --- V3 Benchmarks (multipart/form-data) ---

# V3: Typecheck Trivial
run_benchmark "V3 Typecheck Trivial" \
    "curl -s -X POST ${PROD_URL}/v3/typecheck -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-trivial.tsx' > /dev/null"

# V3: Typecheck Simple Component
run_benchmark "V3 Typecheck Simple" \
    "curl -s -X POST ${PROD_URL}/v3/typecheck -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-simple.tsx' > /dev/null"

# V3: Typecheck Medium Component
run_benchmark "V3 Typecheck Medium" \
    "curl -s -X POST ${PROD_URL}/v3/typecheck -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-medium.tsx' > /dev/null"

# V3: Compile Trivial
run_benchmark "V3 Compile Trivial" \
    "curl -s -X POST ${PROD_URL}/v3/compile -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-trivial.tsx' > /dev/null"

# V3: Compile Simple Component
run_benchmark "V3 Compile Simple" \
    "curl -s -X POST ${PROD_URL}/v3/compile -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-simple.tsx' > /dev/null"

# V3: Compile Medium Component
run_benchmark "V3 Compile Medium" \
    "curl -s -X POST ${PROD_URL}/v3/compile -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-medium.tsx' > /dev/null"

# V3: Compile with skip_typecheck
run_benchmark "V3 Compile Skip Typecheck Medium" \
    "curl -s -X POST '${PROD_URL}/v3/compile?skip_typecheck=true' -F '/bun.lock=<${FIXTURES_DIR}/bench-v3-bun.lock' -F '/package.json=<${FIXTURES_DIR}/bench-v3-package.json' -F '/tsconfig.json=<${FIXTURES_DIR}/bench-v3-tsconfig.json' -F '/index.tsx=<${FIXTURES_DIR}/bench-v3-medium.tsx' > /dev/null"

# --- V1 Benchmarks (JSON) ---

# 2. Simple React Component Build
run_benchmark "Simple React Build" \
    "curl -s -X POST ${PROD_URL}/build -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-simple.json > /dev/null"

# 3. @crayonnow/core Build
run_benchmark "Core Import Build" \
    "curl -s -X POST ${PROD_URL}/build -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-core.json > /dev/null"

# 4. Complex Component Build
run_benchmark "Complex Component Build" \
    "curl -s -X POST ${PROD_URL}/build -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-meditation.json > /dev/null"

# 5. Type Check - Valid Code
run_benchmark "Type Check Valid" \
    "curl -s -X POST ${PROD_URL}/typecheck -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-type-valid.json > /dev/null"

# 6. Type Check - With Error
run_benchmark "Type Check Error" \
    "curl -s -X POST ${PROD_URL}/typecheck -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-type-error.json > /dev/null"

# 7. Build with Type Validation - Valid
run_benchmark "Build with Validation Valid" \
    "curl -s -X POST '${PROD_URL}/build?validate_types=true' -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-type-valid.json > /dev/null"

# 8. Build with Type Validation - Error
run_benchmark "Build with Validation Error" \
    "curl -s -X POST '${PROD_URL}/build?validate_types=true' -H 'Content-Type: application/json' -d @${FIXTURES_DIR}/bench-type-error.json > /dev/null"

# Generate summary report
echo -e "${YELLOW}Generating summary report...${NC}\n"

cat > /tmp/benchmark-report.md << EOF
# TypeScript-Go Production Benchmark Results

## Test Configuration
- **Endpoint**: https://server-wild-sea-9370.fly.dev
- **Runs per test**: 50
- **Warmup runs**: 5
- **Date**: $(date)

## Results

EOF

# Append individual benchmark results
for file in /tmp/bench-*.md; do
    if [ -f "$file" ] && [[ "$file" != "/tmp/benchmark-report.md" ]]; then
        echo "### $(basename "$file" .md | sed 's/bench-//' | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) tolower(substr($i,2))}1')" >> /tmp/benchmark-report.md
        tail -n +2 "$file" >> /tmp/benchmark-report.md
        echo "" >> /tmp/benchmark-report.md
    fi
done

# Print summary
echo -e "${GREEN}=== Benchmark Complete ===${NC}"
echo -e "Summary report saved to: /tmp/benchmark-report.md"
echo -e "Individual JSON results saved to: /tmp/bench-*.json\n"

# Display summary
echo -e "${BLUE}=== Summary ===${NC}"
cat /tmp/benchmark-report.md

# Save to benchmark-results directory
RESULTS_DIR="$SCRIPT_DIR/benchmark-results"
mkdir -p "$RESULTS_DIR"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
cp /tmp/benchmark-report.md "$RESULTS_DIR/$TIMESTAMP.md"
echo -e "${GREEN}Report saved to: $RESULTS_DIR/$TIMESTAMP.md${NC}"

# Also copy JSON results
for file in /tmp/bench-*.json; do
    if [ -f "$file" ]; then
        basename_without_bench="${file#/tmp/bench-}"
        cp "$file" "$RESULTS_DIR/$TIMESTAMP-${basename_without_bench}"
    fi
done
echo -e "${GREEN}JSON results saved to: $RESULTS_DIR/$TIMESTAMP-*.json${NC}"