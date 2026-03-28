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

// singleFileFixture wraps a single code string into v2 multi-file format.
func singleFileFixture(code string) map[string]string {
	return map[string]string{"/index.tsx": code}
}
