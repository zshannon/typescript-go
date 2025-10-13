# TypeScript-Go Server Testing Guide

## Setup

### 1. Prerequisites
- 1Password CLI (`op`) installed and configured
- AWS CLI installed
- Go 1.21+ installed
- Access to S3 bucket credentials via `.env.op`

### 2. Running the Server Locally

```bash
# From the server directory
cd /Users/zcs/Code/zshannon/2025/typescript-go/server

# Start the server with S3 credentials
op run --env-file="../.env.op" -- go run .
```

The server runs on port 8080 with metrics on port 9091.

## Test Cases

### 1. Health Check
```bash
curl -s http://localhost:8080/health | jq
```

**Expected Output:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "3s",
  "cache_size": "32 MB",
  "cache_entries": 2663
}
```

### 2. Simple React Component

Create test payload:
```bash
cat > /tmp/test-simple.json << 'EOF'
{
  "code": "import React from 'react';\nexport default () => React.createElement('div', null, 'Hello');",
  "version": "0.0.4"
}
EOF
```

Test build:
```bash
curl -s -X POST http://localhost:8080/build \
  -H "Content-Type: application/json" \
  -d @/tmp/test-simple.json | jq
```

**Expected:** Returns `{"code": "..."}` with compiled JavaScript.

### 3. @crayonnow/core Import

Create test payload:
```bash
cat > /tmp/test-core.json << 'EOF'
{
  "code": "import { Text } from '@crayonnow/core';\nexport default () => <Text>Hello</Text>;",
  "version": "0.0.4"
}
EOF
```

Test build:
```bash
curl -s -X POST http://localhost:8080/build \
  -H "Content-Type: application/json" \
  -d @/tmp/test-core.json | jq keys
```

**Expected:** `["code"]` - Successfully builds with @crayonnow/core dependencies.

### 4. Complex Component (Meditation Timer)

Create test payload:
```bash
cat > /tmp/test-meditation.json << 'EOF'
{
  "code": "import { Flex, Text, Button, Picker } from '@crayonnow/core';\nimport { useState, useEffect } from 'react';\n\nexport default () => {\n  const [duration, setDuration] = useState('5');\n  const [seconds, setSeconds] = useState(0);\n  const [running, setRunning] = useState(false);\n\n  const handleStartPause = () => {\n    if (running) {\n      setRunning(false);\n    } else {\n      if (seconds === 0) {\n        setSeconds(parseInt(duration, 10) * 60);\n      }\n      setRunning(true);\n    }\n  };\n\n  const handleReset = () => {\n    setRunning(false);\n    setSeconds(parseInt(duration, 10) * 60);\n  };\n\n  useEffect(() => {\n    if (running && seconds > 0) {\n      const timer = setTimeout(() => setSeconds(seconds - 1), 1000);\n      return () => clearTimeout(timer);\n    }\n    if (running && seconds === 0) {\n      setRunning(false);\n    }\n  }, [running, seconds]);\n\n  const minutesDisplay = String(Math.floor(seconds / 60)).padStart(2, '0');\n  const secondsDisplay = String(seconds % 60).padStart(2, '0');\n\n  return (\n    <Flex style={{ alignItems: 'stretch', minHeight: '100vh', background: '#e0f7fa', padding: '20px', rowGap: '16px' }}>\n      <Text style={{ fontSize: '24px', fontWeight: '600', textAlign: 'center', color: '#006064' }}>\n        Meditation Timer\n      </Text>\n      <Picker\n        value={duration}\n        onChange={(val) => {\n          setDuration(val);\n          setSeconds(parseInt(val, 10) * 60);\n          setRunning(false);\n        }}\n        style={{ background: 'white', borderRadius: '8px', padding: '12px' }}\n      >\n        <Text value=\"5\">5 Minutes</Text>\n        <Text value=\"10\">10 Minutes</Text>\n        <Text value=\"15\">15 Minutes</Text>\n        <Text value=\"20\">20 Minutes</Text>\n      </Picker>\n      <Text style={{ fontSize: '48px', fontWeight: 'bold', textAlign: 'center', color: '#004d40' }}>\n        {minutesDisplay}:{secondsDisplay}\n      </Text>\n      <Button onClick={handleStartPause} style={{ background: running ? '#ff9800' : '#34C759', borderRadius: '8px', padding: '12px' }}>\n        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>{running ? 'Pause' : 'Start'}</Text>\n      </Button>\n      <Button onClick={handleReset} style={{ background: '#f44336', borderRadius: '8px', padding: '12px' }}>\n        <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>Reset</Text>\n      </Button>\n    </Flex>\n  );\n};",
  "version": "0.0.4"
}
EOF
```

Test build:
```bash
# Check if it builds successfully (should return >100KB)
curl -s -X POST http://localhost:8080/build \
  -H "Content-Type: application/json" \
  -d @/tmp/test-meditation.json | jq -r .code | wc -c
```

**Expected:** Returns ~140KB of compiled JavaScript.

### 5. Type Checking

```bash
# Test with type error
cat > /tmp/test-type-error.json << 'EOF'
{
  "code": "export const hello: string = 123",
  "version": "0.0.4"
}
EOF

curl -s -X POST http://localhost:8080/typecheck \
  -H "Content-Type: application/json" \
  -d @/tmp/test-type-error.json | jq
```

**Expected Output:**
```json
{
  "errors": [
    {
      "message": "Type 'number' is not assignable to type 'string'.",
      "line": 1,
      "column": 30
    }
  ]
}
```

### 6. Build with Type Validation

```bash
curl -s -X POST "http://localhost:8080/build?validate_types=true" \
  -H "Content-Type: application/json" \
  -d @/tmp/test-type-error.json | jq
```

**Expected:** Returns type errors, not compiled code.

## Production Testing

Test against deployed server:
```bash
# Replace localhost:8080 with production URL
curl -s -X POST https://server-wild-sea-9370.fly.dev/build \
  -H "Content-Type: application/json" \
  -d @/tmp/test-meditation.json | jq -r .code | wc -c
```

## Cache Management

### Flush Cache
```bash
curl -s -X POST http://localhost:8080/flush-cache | jq
```

**Expected Output:**
```json
{
  "status": "success",
  "message": "Cache flushed successfully",
  "entries_cleared": 238,
  "timestamp": 1756255182
}
```

## Troubleshooting

### Common Issues

1. **"file not found" errors for npm packages**
   - Ensure packages are uploaded to S3 bucket
   - Check S3 bucket contents: `op run --env-file="../.env.op" -- aws s3 ls s3://fly-tsgo-node-modules/0.0.4/node_modules/ --endpoint-url="$AWS_ENDPOINT_URL_S3"`
   - Flush cache after uploading new packages

2. **Port already in use**
   ```bash
   # Kill processes on ports
   lsof -ti:8080 | xargs kill -9 2>/dev/null
   lsof -ti:9091 | xargs kill -9 2>/dev/null
   ```

3. **Module resolution errors**
   - Enable logging by uncommenting log statements in server.go
   - Check logs for resolution paths: `grep "OnResolve" /tmp/server.log`

## Performance Benchmarks

```bash
# Local testing
hyperfine --warmup 3 --min-runs 10 \
  'curl -s -X POST http://localhost:8080/typecheck -H "Content-Type: application/json" -d @/tmp/test-meditation.json' \
  'curl -s -X POST http://localhost:8080/build -H "Content-Type: application/json" -d @/tmp/test-meditation.json'
```

**Expected Performance:**
- Typecheck: ~200ms (including network)
- Build: ~150ms (including network)
- Local: ~10-15ms