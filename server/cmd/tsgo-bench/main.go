package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"flag"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	_ "embed"

	"github.com/dadrus/httpsig"
	"github.com/mr-tron/base58"
)

// Shared config files for all fixtures — matches the real crayon tsgo deployment.
const packageJSON = `{
  "main": "./index.tsx",
  "dependencies": {
    "@flickfyi/core": "0.0.8",
    "@flickfyi/lens": "0.0.0",
    "@flickfyi/photon": "0.0.2",
    "@react-spring/core": "10.0.3",
    "@react-spring/shared": "10.0.3",
    "@types/react": "19.2.8",
    "@use-gesture/react": "10.3.1",
    "lodash": "4.17.21",
    "react": "19.2.3",
    "rxjs": "7.8.2",
    "typescript": "5.9.3",
    "zod": "4.3.5"
  },
  "resolve-s3": ["@flickfyi/core", "@flickfyi/lens", "@flickfyi/photon"]
}`

const tsconfigJSON = `{"compilerOptions": {"strict": true, "jsx": "react-jsx", "jsxImportSource": "@flickfyi/core", "target": "ES2022", "module": "commonjs", "moduleResolution": "bundler", "lib": ["ES2022"]}}`

// Real bun.lock from crayon repo — only covers public npm packages.
// Private @flickfyi/* packages are pre-seeded from S3 via resolve-s3.
//
//go:embed bunlock.txt
var bunLock string

// Fixture definitions.

type fixture struct {
	files map[string]string
	name  string
}

var fixtures = []fixture{
	{
		name:  "Trivial",
		files: map[string]string{"/index.tsx": `export const x: number = 1;`},
	},
	{
		name: "SmallComponent",
		files: map[string]string{"/index.tsx": `import { Text } from '@flickfyi/core';

interface GreetingProps {
  name: string;
}

export default ({ name }: GreetingProps) => (
  <Text style={{ fontSize: '18px', fontWeight: '600' }}>
    Hello, {name}!
  </Text>
);`},
	},
	{
		name: "MediumComponent",
		files: map[string]string{"/index.tsx": `import { Flex, Text, Button, Picker } from '@flickfyi/core';
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
};`},
	},
	{
		name: "MultiFile",
		files: map[string]string{
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
			"/Button.tsx": `import { Button as CoreButton, Text } from '@flickfyi/core';

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
			"/Card.tsx": `import { Flex, Text } from '@flickfyi/core';
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
			"/index.tsx": `import { Flex, Text, Picker } from '@flickfyi/core';
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
		},
	},
	{
		name: "TypeError",
		files: map[string]string{"/index.tsx": `export const x: string = 123;`},
	},
	{
		name: "MultiError",
		files: map[string]string{
			"/index.tsx": `import { foo } from './lib';
export const x: string = foo;`,
			"/lib.ts": `export const foo: number = 42;`,
		},
	},
}

// endpoint defines a server endpoint to benchmark.
type endpoint struct {
	name string
	path string
}

var endpoints = []endpoint{
	{name: "/v3/compile", path: "/v3/compile?skip_typecheck=true"},
	{name: "/v3/compile+tc", path: "/v3/compile"},
	{name: "/v3/typecheck", path: "/v3/typecheck"},
}

func main() {
	keyFlag := flag.String("key", "", "Base58-encoded P-256 private key for request signing")
	runsFlag := flag.Int("runs", 10, "Number of measured runs per benchmark")
	urlFlag := flag.String("url", "", "Base URL of the tsgo server (required)")
	flag.Parse()

	if *urlFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: tsgo-bench --url <server-url> [--key <base58-privkey>] [--runs N]")
		os.Exit(1)
	}

	baseURL := strings.TrimRight(*urlFlag, "/")
	runs := *runsFlag
	warmups := 2

	var privKey *ecdsa.PrivateKey
	if *keyFlag != "" {
		var err error
		privKey, err = parsePrivateKey(*keyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid --key: %v\n", err)
			os.Exit(1)
		}
	}

	authLabel := "disabled"
	if privKey != nil {
		authLabel = "enabled"
	}

	fmt.Println("tsgo-bench v3 benchmarks")
	fmt.Printf("Target: %s\n", baseURL)
	fmt.Printf("Auth:   %s\n", authLabel)
	fmt.Printf("Runs:   %d (+ %d warmup)\n\n", runs, warmups)

	fmt.Printf("%-20s %-16s %-10s %-10s %-10s\n", "Fixture", "Endpoint", "Avg", "Min", "Max")

	for _, fix := range fixtures {
		body, contentType := buildMultipart(fix.files)

		for _, ep := range endpoints {
			durations := make([]time.Duration, 0, runs)
			url := baseURL + ep.path
			skipped := false

			// Warmup runs
			for i := 0; i < warmups; i++ {
				_, err := sendRequest(url, contentType, body, privKey, *keyFlag)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [warmup] %s %s: %v\n", fix.name, ep.name, err)
					skipped = true
					break
				}
			}
			if skipped {
				continue
			}

			// Measured runs
			for i := 0; i < runs; i++ {
				d, err := sendRequest(url, contentType, body, privKey, *keyFlag)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [run %d] %s %s: %v\n", i+1, fix.name, ep.name, err)
					skipped = true
					break
				}
				durations = append(durations, d)
			}
			if skipped {
				continue
			}

			avg, min, max := stats(durations)
			fmt.Printf("%-20s %-16s %-10s %-10s %-10s\n",
				fix.name, ep.name,
				fmtDuration(avg), fmtDuration(min), fmtDuration(max))
		}
	}
}

// parsePrivateKey decodes a base58 raw scalar into an ecdsa.PrivateKey on P-256.
func parsePrivateKey(encoded string) (*ecdsa.PrivateKey, error) {
	decoded, err := base58.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("base58 decode: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("expected 32 bytes, got %d", len(decoded))
	}

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()},
		D:        new(big.Int).SetBytes(decoded),
	}
	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(priv.D.Bytes())
	return priv, nil
}

// buildMultipart creates a multipart/form-data body from fixture files plus shared config.
func buildMultipart(files map[string]string) ([]byte, string) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	// Config files first (alphabetical)
	fw, _ := w.CreateFormFile("/bun.lock", "/bun.lock")
	fw.Write([]byte(bunLock))
	fw, _ = w.CreateFormFile("/package.json", "/package.json")
	fw.Write([]byte(packageJSON))
	fw, _ = w.CreateFormFile("/tsconfig.json", "/tsconfig.json")
	fw.Write([]byte(tsconfigJSON))

	// Source files in sorted order for deterministic bodies
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		fw, _ = w.CreateFormFile(p, p)
		fw.Write([]byte(files[p]))
	}

	w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

// sendRequest sends a POST to the given URL, optionally signing it, and returns the round-trip duration.
func sendRequest(url, contentType string, body []byte, privKey *ecdsa.PrivateKey, keyID string) (time.Duration, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	if privKey != nil {
		if err := signHTTPRequest(req, privKey, keyID); err != nil {
			return 0, fmt.Errorf("signing request: %w", err)
		}
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Drain the body to ensure we measure the full response
	io.Copy(io.Discard, resp.Body)
	return elapsed, nil
}

// signHTTPRequest signs a request using RFC 9421 HTTP Message Signatures via dadrus/httpsig.
func signHTTPRequest(req *http.Request, privKey *ecdsa.PrivateKey, keyID string) error {
	// Compute the compressed public key for the KeyID
	compressed := elliptic.MarshalCompressed(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	pubKeyBase58 := base58.Encode(compressed)

	key := httpsig.Key{
		Algorithm: httpsig.EcdsaP256Sha256,
		Key:       privKey,
		KeyID:     pubKeyBase58,
	}

	signer, err := httpsig.NewSigner(key,
		httpsig.WithComponents("@method", "@path", "@authority", "content-digest"),
		httpsig.WithContentDigestAlgorithm(httpsig.Sha256),
	)
	if err != nil {
		return fmt.Errorf("creating signer: %w", err)
	}

	msg := httpsig.MessageFromRequest(req)
	headers, err := signer.Sign(msg)
	if err != nil {
		return fmt.Errorf("signing: %w", err)
	}

	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}
	return nil
}

// stats returns avg, min, max from a slice of durations.
func stats(durations []time.Duration) (avg, min, max time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0
	}
	min = durations[0]
	max = durations[0]
	var total time.Duration
	for _, d := range durations {
		total += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	avg = total / time.Duration(len(durations))
	return avg, min, max
}

// fmtDuration formats a duration as e.g. "12.3ms" or "1.23s".
func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
