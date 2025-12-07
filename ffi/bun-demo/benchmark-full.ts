#!/usr/bin/env bun
/**
 * Full Benchmark: Type Checking (FFI) + Bundling (Bun)
 *
 * This benchmark compares:
 * 1. Type checking via FFI (TypeScript-Go)
 * 2. Bundling via Bun.build()
 * 3. Combined typecheck + bundle pipeline
 */

import tsgo from "./tsgo";
import { mkdtemp, writeFile, rm } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";

console.log("=".repeat(70));
console.log("Full Benchmark: Type Checking (FFI) + Bundling (Bun)");
console.log("=".repeat(70));
console.log(`\ntsgo version: ${tsgo.version()}`);
console.log(`Bun version: ${Bun.version}\n`);

// React component for testing
const reactComponent = `
import React, { useState, useCallback, useMemo } from 'react';

interface Todo {
  id: number;
  text: string;
  completed: boolean;
  createdAt: Date;
}

interface TodoListProps {
  initialTodos?: Todo[];
  onTodoChange?: (todos: Todo[]) => void;
}

type FilterType = 'all' | 'active' | 'completed';

const TodoList: React.FC<TodoListProps> = ({ initialTodos = [], onTodoChange }) => {
  const [todos, setTodos] = useState<Todo[]>(initialTodos);
  const [filter, setFilter] = useState<FilterType>('all');
  const [newTodoText, setNewTodoText] = useState('');

  const filteredTodos = useMemo(() => {
    switch (filter) {
      case 'active':
        return todos.filter(t => !t.completed);
      case 'completed':
        return todos.filter(t => t.completed);
      default:
        return todos;
    }
  }, [todos, filter]);

  const addTodo = useCallback(() => {
    if (!newTodoText.trim()) return;

    const newTodo: Todo = {
      id: Date.now(),
      text: newTodoText,
      completed: false,
      createdAt: new Date(),
    };

    const updatedTodos = [...todos, newTodo];
    setTodos(updatedTodos);
    setNewTodoText('');
    onTodoChange?.(updatedTodos);
  }, [newTodoText, todos, onTodoChange]);

  const toggleTodo = useCallback((id: number) => {
    const updatedTodos = todos.map(t =>
      t.id === id ? { ...t, completed: !t.completed } : t
    );
    setTodos(updatedTodos);
    onTodoChange?.(updatedTodos);
  }, [todos, onTodoChange]);

  const deleteTodo = useCallback((id: number) => {
    const updatedTodos = todos.filter(t => t.id !== id);
    setTodos(updatedTodos);
    onTodoChange?.(updatedTodos);
  }, [todos, onTodoChange]);

  return (
    <div className="todo-list">
      <h1>Todo List</h1>
      <div className="add-todo">
        <input
          type="text"
          value={newTodoText}
          onChange={e => setNewTodoText(e.target.value)}
          onKeyPress={e => e.key === 'Enter' && addTodo()}
          placeholder="Add a new todo..."
        />
        <button onClick={addTodo}>Add</button>
      </div>
      <div className="filters">
        <button onClick={() => setFilter('all')} disabled={filter === 'all'}>All</button>
        <button onClick={() => setFilter('active')} disabled={filter === 'active'}>Active</button>
        <button onClick={() => setFilter('completed')} disabled={filter === 'completed'}>Completed</button>
      </div>
      <ul>
        {filteredTodos.map(todo => (
          <li key={todo.id} className={todo.completed ? 'completed' : ''}>
            <input
              type="checkbox"
              checked={todo.completed}
              onChange={() => toggleTodo(todo.id)}
            />
            <span>{todo.text}</span>
            <button onClick={() => deleteTodo(todo.id)}>Delete</button>
          </li>
        ))}
      </ul>
      <div className="stats">
        <span>{todos.filter(t => !t.completed).length} items left</span>
      </div>
    </div>
  );
};

export default TodoList;
`;

// Simple TypeScript for comparison
const simpleTs = `
interface User {
  id: number;
  name: string;
  email: string;
}

function createUser(name: string, email: string): User {
  return { id: Math.random(), name, email };
}

export const user = createUser("Alice", "alice@example.com");
`;

// Benchmark helpers
interface BenchResult {
  name: string;
  iterations: number;
  totalMs: number;
  avgMs: number;
  minMs: number;
  maxMs: number;
  opsPerSec: number;
}

async function benchmark(
  name: string,
  fn: () => Promise<void> | void,
  iterations: number = 50
): Promise<BenchResult> {
  // Warmup
  for (let i = 0; i < 3; i++) {
    await fn();
  }

  const times: number[] = [];
  for (let i = 0; i < iterations; i++) {
    const start = performance.now();
    await fn();
    times.push(performance.now() - start);
  }

  const totalMs = times.reduce((a, b) => a + b, 0);
  const avgMs = totalMs / iterations;
  const minMs = Math.min(...times);
  const maxMs = Math.max(...times);

  return {
    name,
    iterations,
    totalMs,
    avgMs,
    minMs,
    maxMs,
    opsPerSec: 1000 / avgMs,
  };
}

// Create temp directory for bundling tests
const tempDir = await mkdtemp(join(tmpdir(), "tsgo-bench-"));

async function cleanup() {
  try {
    await rm(tempDir, { recursive: true });
  } catch {}
}

process.on("exit", cleanup);
process.on("SIGINT", cleanup);

// Write test files for bundling
const entryFile = join(tempDir, "entry.tsx");
await writeFile(entryFile, reactComponent);

const simpleFile = join(tempDir, "simple.ts");
await writeFile(simpleFile, simpleTs);

console.log("Running benchmarks...\n");

const results: BenchResult[] = [];

// 1. Type checking benchmarks
console.log("--- Type Checking (FFI) ---");

const typecheckSimple = await benchmark(
  "typecheck-simple",
  () => {
    tsgo.typecheck(simpleTs, "/project/simple.ts");
  },
  100
);
results.push(typecheckSimple);
console.log(`${typecheckSimple.name}: ${typecheckSimple.avgMs.toFixed(2)}ms avg (${typecheckSimple.opsPerSec.toFixed(1)} ops/sec)`);

const typecheckReact = await benchmark(
  "typecheck-react",
  () => {
    tsgo.typecheck(reactComponent, "/project/TodoList.tsx");
  },
  50
);
results.push(typecheckReact);
console.log(`${typecheckReact.name}: ${typecheckReact.avgMs.toFixed(2)}ms avg (${typecheckReact.opsPerSec.toFixed(1)} ops/sec)`);

// 2. Bundling benchmarks (Bun.build)
console.log("\n--- Bundling (Bun.build) ---");

const bundleSimple = await benchmark(
  "bundle-simple",
  async () => {
    await Bun.build({
      entrypoints: [simpleFile],
      outdir: tempDir,
      naming: "simple-out.js",
      minify: true,
      target: "browser",
    });
  },
  100
);
results.push(bundleSimple);
console.log(`${bundleSimple.name}: ${bundleSimple.avgMs.toFixed(2)}ms avg (${bundleSimple.opsPerSec.toFixed(1)} ops/sec)`);

const bundleReact = await benchmark(
  "bundle-react",
  async () => {
    await Bun.build({
      entrypoints: [entryFile],
      outdir: tempDir,
      naming: "react-out.js",
      minify: true,
      target: "browser",
      external: ["react", "react-dom"],
    });
  },
  50
);
results.push(bundleReact);
console.log(`${bundleReact.name}: ${bundleReact.avgMs.toFixed(2)}ms avg (${bundleReact.opsPerSec.toFixed(1)} ops/sec)`);

// 3. Combined pipeline: typecheck + bundle
console.log("\n--- Combined Pipeline (Typecheck + Bundle) ---");

const pipelineSimple = await benchmark(
  "pipeline-simple",
  async () => {
    // Type check first
    const result = tsgo.typecheck(simpleTs, "/project/simple.ts");
    if (!result.success) throw new Error("Type check failed");

    // Then bundle
    await Bun.build({
      entrypoints: [simpleFile],
      outdir: tempDir,
      naming: "pipeline-simple.js",
      minify: true,
      target: "browser",
    });
  },
  50
);
results.push(pipelineSimple);
console.log(`${pipelineSimple.name}: ${pipelineSimple.avgMs.toFixed(2)}ms avg (${pipelineSimple.opsPerSec.toFixed(1)} ops/sec)`);

const pipelineReact = await benchmark(
  "pipeline-react",
  async () => {
    // Type check first
    const result = tsgo.typecheck(reactComponent, "/project/TodoList.tsx");
    if (!result.success) throw new Error("Type check failed");

    // Then bundle
    await Bun.build({
      entrypoints: [entryFile],
      outdir: tempDir,
      naming: "pipeline-react.js",
      minify: true,
      target: "browser",
      external: ["react", "react-dom"],
    });
  },
  30
);
results.push(pipelineReact);
console.log(`${pipelineReact.name}: ${pipelineReact.avgMs.toFixed(2)}ms avg (${pipelineReact.opsPerSec.toFixed(1)} ops/sec)`);

// 4. Bun's transpileOnly for comparison (no type checking)
console.log("\n--- Bun Transpile Only (no types) ---");

const transpileSimple = await benchmark(
  "transpile-simple",
  () => {
    new Bun.Transpiler({ loader: "ts" }).transformSync(simpleTs);
  },
  500
);
results.push(transpileSimple);
console.log(`${transpileSimple.name}: ${transpileSimple.avgMs.toFixed(2)}ms avg (${transpileSimple.opsPerSec.toFixed(1)} ops/sec)`);

const transpileReact = await benchmark(
  "transpile-react",
  () => {
    new Bun.Transpiler({ loader: "tsx" }).transformSync(reactComponent);
  },
  500
);
results.push(transpileReact);
console.log(`${transpileReact.name}: ${transpileReact.avgMs.toFixed(2)}ms avg (${transpileReact.opsPerSec.toFixed(1)} ops/sec)`);

// Summary
console.log("\n" + "=".repeat(70));
console.log("Summary");
console.log("=".repeat(70));
console.log("\n| Benchmark                    | Avg (ms) | Min (ms) | Max (ms) | Ops/sec |");
console.log("|------------------------------|----------|----------|----------|---------|");
for (const r of results) {
  console.log(
    `| ${r.name.padEnd(28)} | ${r.avgMs.toFixed(2).padStart(8)} | ${r.minMs.toFixed(2).padStart(8)} | ${r.maxMs.toFixed(2).padStart(8)} | ${r.opsPerSec.toFixed(1).padStart(7)} |`
  );
}

// Analysis
console.log("\n" + "=".repeat(70));
console.log("Analysis");
console.log("=".repeat(70));

const typecheckTime = typecheckReact.avgMs;
const bundleTime = bundleReact.avgMs;
const pipelineTime = pipelineReact.avgMs;
const transpileTime = transpileReact.avgMs;

console.log(`
React Component (${reactComponent.split('\n').length} lines):
  - Type checking (FFI):     ${typecheckTime.toFixed(2)}ms
  - Bundling (Bun.build):    ${bundleTime.toFixed(2)}ms
  - Full pipeline:           ${pipelineTime.toFixed(2)}ms
  - Transpile only (no TS):  ${transpileTime.toFixed(2)}ms

Type checking overhead: ${(typecheckTime / transpileTime).toFixed(1)}x slower than transpile-only
Full pipeline overhead: ${(pipelineTime / bundleTime).toFixed(1)}x slower than bundle-only
`);

// Cleanup
await cleanup();

console.log("=".repeat(70));
console.log("Benchmark complete!");
console.log("=".repeat(70));
