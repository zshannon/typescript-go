# Server V2 Benchmark Suite Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add layered Go benchmarks for the v2 server endpoints that isolate diskFS setup, TypeScript compilation, esbuild bundling, and HTTP overhead.

**Architecture:** Two new test files in `server/`: `bench_fixtures_test.go` (fixture constants) and `bench_v2_test.go` (benchmarks + setup). Three benchmark layers: direct function calls for typecheck, direct function calls for build, and HTTP handler benchmarks. All share a `setupBenchServer` that pre-syncs mock S3 and silences logs.

**Tech Stack:** Go `testing.B`, `net/http/httptest`, existing `MockS3Client` from `test_utils.go`

**Spec:** `docs/superpowers/specs/2026-03-28-server-benchmarks-design.md`

---

## Chunk 1: Fixtures and Setup

### Task 1: Create bench_fixtures_test.go

**Files:**
- Create: `server/bench_fixtures_test.go`

- [ ] **Step 1: Write the fixtures file**

```go
package main

const benchVersion = "0.0.4"

// Trivial — baseline overhead measurement
const fixtureTrivial = `export const x: number = 1;`

// Small — simple React component with @crayonnow/core
const fixtureSmallComponent = `import { Text } from '@crayonnow/core';

interface GreetingProps {
  name: string;
}

export default ({ name }: GreetingProps) => (
  <Text style={{ fontSize: '18px', fontWeight: '600' }}>
    Hello, {name}!
  </Text>
);`

// Medium — meditation timer (verbatim from production test fixtures)
const fixtureMediumComponent = `import { Flex, Text, Button, Picker } from '@crayonnow/core';
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
};`

// Multi-file project — 5 interconnected files
var fixtureMultiFile = map[string]string{
	"/types.ts": `export interface TimerState {
  seconds: number;
  running: boolean;
}

export interface TimerConfig {
  duration: number;
  label: string;
}

export const DEFAULT_CONFIGS: TimerConfig[] = [
  { duration: 5, label: '5 Minutes' },
  { duration: 10, label: '10 Minutes' },
  { duration: 15, label: '15 Minutes' },
  { duration: 20, label: '20 Minutes' },
];`,

	"/hooks.ts": `import { useState, useEffect } from 'react';
import { TimerState } from './types';

export function useTimer(initialDuration: number): TimerState & {
  start: () => void;
  pause: () => void;
  reset: (duration: number) => void;
} {
  const [seconds, setSeconds] = useState(initialDuration * 60);
  const [running, setRunning] = useState(false);

  useEffect(() => {
    if (running && seconds > 0) {
      const timer = setTimeout(() => setSeconds(s => s - 1), 1000);
      return () => clearTimeout(timer);
    }
    if (running && seconds === 0) {
      setRunning(false);
    }
  }, [running, seconds]);

  return {
    seconds,
    running,
    start: () => { if (seconds > 0) setRunning(true); },
    pause: () => setRunning(false),
    reset: (duration: number) => { setRunning(false); setSeconds(duration * 60); },
  };
}`,

	"/Button.tsx": `import { Button as CoreButton, Text } from '@crayonnow/core';

interface TimerButtonProps {
  label: string;
  color: string;
  onPress: () => void;
}

export default ({ color, label, onPress }: TimerButtonProps) => (
  <CoreButton onClick={onPress} style={{ background: color, borderRadius: '8px', padding: '12px' }}>
    <Text style={{ color: 'white', textAlign: 'center', fontWeight: '600' }}>{label}</Text>
  </CoreButton>
);`,

	"/Card.tsx": `import { Flex, Text } from '@crayonnow/core';
import TimerButton from './Button';
import { TimerConfig } from './types';

interface CardProps {
  config: TimerConfig;
  display: string;
  onReset: () => void;
  onToggle: () => void;
  running: boolean;
}

export default ({ config, display, onReset, onToggle, running }: CardProps) => (
  <Flex style={{ background: 'white', borderRadius: '12px', padding: '16px', rowGap: '12px' }}>
    <Text style={{ fontSize: '18px', fontWeight: '600' }}>{config.label}</Text>
    <Text style={{ fontSize: '36px', fontWeight: 'bold', textAlign: 'center' }}>{display}</Text>
    <TimerButton label={running ? 'Pause' : 'Start'} color={running ? '#ff9800' : '#34C759'} onPress={onToggle} />
    <TimerButton label="Reset" color="#f44336" onPress={onReset} />
  </Flex>
);`,

	"/index.tsx": `import { Flex, Text, Picker } from '@crayonnow/core';
import { useState } from 'react';
import { DEFAULT_CONFIGS } from './types';
import { useTimer } from './hooks';
import Card from './Card';

export default () => {
  const [configIdx, setConfigIdx] = useState(0);
  const config = DEFAULT_CONFIGS[configIdx];
  const timer = useTimer(config.duration);

  const minutesDisplay = String(Math.floor(timer.seconds / 60)).padStart(2, '0');
  const secondsDisplay = String(timer.seconds % 60).padStart(2, '0');
  const display = minutesDisplay + ':' + secondsDisplay;

  return (
    <Flex style={{ alignItems: 'stretch', minHeight: '100vh', background: '#e0f7fa', padding: '20px', rowGap: '16px' }}>
      <Text style={{ fontSize: '24px', fontWeight: '600', textAlign: 'center', color: '#006064' }}>
        Meditation Timer
      </Text>
      <Picker
        value={String(configIdx)}
        onChange={(val) => { const idx = parseInt(val, 10); setConfigIdx(idx); timer.reset(DEFAULT_CONFIGS[idx].duration); }}
        style={{ background: 'white', borderRadius: '8px', padding: '12px' }}
      >
        {DEFAULT_CONFIGS.map((c, i) => (
          <Text key={i} value={String(i)}>{c.label}</Text>
        ))}
      </Picker>
      <Card
        config={config}
        display={display}
        running={timer.running}
        onToggle={() => timer.running ? timer.pause() : timer.start()}
        onReset={() => timer.reset(config.duration)}
      />
    </Flex>
  );
};`,
}

// Helper: wrap a single-file fixture into v2 multi-file format
func singleFileFixture(code string) map[string]string {
	return map[string]string{"/index.tsx": code}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd server && go vet ./...`
Expected: clean (no errors)

- [ ] **Step 3: Commit**

```bash
git add server/bench_fixtures_test.go
git commit -m "Add benchmark fixture constants for v2 endpoints"
```

### Task 2: Create bench_v2_test.go with setupBenchServer

**Files:**
- Create: `server/bench_v2_test.go`

- [ ] **Step 1: Write the setup function and a smoke-test benchmark**

Write `bench_v2_test.go` with `setupBenchServer` and one minimal benchmark (`BenchmarkV2Typecheck/Trivial`) to verify the setup works.

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// setupBenchServer initializes the server with mock S3 and pre-synced disk cache.
// Silences log output during benchmarks. Must be called once per top-level benchmark.
func setupBenchServer(b *testing.B) {
	b.Helper()

	mockS3 := NewMockS3Client()
	s3Client = mockS3
	s3Bucket = "test-bucket"
	serverVersion = "1.0.0"
	startTime = time.Now()

	tmpDir, err := os.MkdirTemp("", "bench-cache-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	diskCachePath = tmpDir

	// Pre-sync: trigger S3 download so iterations only see os.Stat
	_, err = newDiskFS(context.Background(), benchVersion)
	if err != nil {
		b.Fatalf("Failed to pre-sync diskFS: %v", err)
	}

	// Silence logs
	origWriter := log.Writer()
	log.SetOutput(io.Discard)

	b.Cleanup(func() {
		log.SetOutput(origWriter)
		os.RemoveAll(tmpDir)
	})
}

func BenchmarkV2Typecheck(b *testing.B) {
	setupBenchServer(b)

	cases := []struct {
		name        string
		files       map[string]string
		entryPoints []string
	}{
		{"Trivial", singleFileFixture(fixtureTrivial), []string{"/index.tsx"}},
		{"SmallComponent", singleFileFixture(fixtureSmallComponent), []string{"/index.tsx"}},
		{"MediumComponent", singleFileFixture(fixtureMediumComponent), []string{"/index.tsx"}},
		{"MultiFile", fixtureMultiFile, []string{"/index.tsx"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resp := typecheckTypeScriptV2(tc.files, tc.entryPoints, benchVersion)
				// Sanity: should not have sync errors
				if len(resp.Errors) > 0 && resp.Errors[0].Message == "failed to sync version" {
					b.Fatalf("Unexpected sync error: %s", resp.Errors[0].Message)
				}
			}
		})
	}
}

func BenchmarkV2Build(b *testing.B) {
	setupBenchServer(b)

	cases := []struct {
		name       string
		files      map[string]string
		entryPoint string
	}{
		{"Trivial", singleFileFixture(fixtureTrivial), "/index.tsx"},
		{"SmallComponent", singleFileFixture(fixtureSmallComponent), "/index.tsx"},
		{"MediumComponent", singleFileFixture(fixtureMediumComponent), "/index.tsx"},
		{"MultiFile", fixtureMultiFile, "/index.tsx"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var lastOutputBytes int
			for i := 0; i < b.N; i++ {
				resp := buildTypeScriptV2(tc.files, tc.entryPoint, benchVersion)
				if len(resp.Errors) > 0 && resp.Errors[0].Message == "failed to sync version" {
					b.Fatalf("Unexpected sync error: %s", resp.Errors[0].Message)
				}
				lastOutputBytes = len(resp.Code)
			}
			b.ReportMetric(float64(lastOutputBytes), "output_bytes/op")
		})
	}
}

func BenchmarkV2HTTP(b *testing.B) {
	setupBenchServer(b)

	type httpCase struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
		body    []byte
	}

	// Pre-serialize request bodies
	mediumTypecheckBody, _ := json.Marshal(TypecheckV2Request{
		Files:       singleFileFixture(fixtureMediumComponent),
		EntryPoints: []string{"/index.tsx"},
		Version:     benchVersion,
	})
	multiFileTypecheckBody, _ := json.Marshal(TypecheckV2Request{
		Files:       fixtureMultiFile,
		EntryPoints: []string{"/index.tsx"},
		Version:     benchVersion,
	})
	mediumBuildBody, _ := json.Marshal(BuildV2Request{
		Files:      singleFileFixture(fixtureMediumComponent),
		EntryPoint: "/index.tsx",
		Version:    benchVersion,
	})
	multiFileBuildBody, _ := json.Marshal(BuildV2Request{
		Files:      fixtureMultiFile,
		EntryPoint: "/index.tsx",
		Version:    benchVersion,
	})

	cases := []httpCase{
		{"Typecheck/MediumComponent", "POST", "/v2/typecheck", typecheckV2, mediumTypecheckBody},
		{"Typecheck/MultiFile", "POST", "/v2/typecheck", typecheckV2, multiFileTypecheckBody},
		{"Build/MediumComponent", "POST", "/v2/build", buildV2, mediumBuildBody},
		{"Build/MultiFile", "POST", "/v2/build", buildV2, multiFileBuildBody},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				tc.handler(w, req)
				if w.Code != http.StatusOK {
					b.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
				}
			}
		})
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd server && go vet ./...`
Expected: clean

- [ ] **Step 3: Run the benchmarks (short, just to verify they work)**

Run: `cd server && go test -bench=BenchmarkV2 -benchmem -benchtime=1s -count=1 -run=^$ ./... 2>/dev/null`
Expected: All benchmarks run and produce output lines like `BenchmarkV2Typecheck/Trivial-16    NNNN    NNNN ns/op    NNNN B/op    NNNN allocs/op`

- [ ] **Step 4: Run the full benchmark suite (3 rounds for stability)**

Run: `cd server && go test -bench=BenchmarkV2 -benchmem -benchtime=5s -count=3 -run=^$ ./... 2>/dev/null`
Expected: All benchmarks produce stable results across 3 rounds

- [ ] **Step 5: Commit**

```bash
git add server/bench_v2_test.go
git commit -m "Add layered v2 benchmark suite (typecheck, build, HTTP)"
```
