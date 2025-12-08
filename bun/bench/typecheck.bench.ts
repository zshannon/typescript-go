/**
 * Benchmarks for tsgo type checker
 *
 * Run with: bun run bench/typecheck.bench.ts
 */

import tsgo from "../src/index";

// Sample code for benchmarks
const samples = {
  simple: `
    const x: number = 42;
    const y: string = "hello";
    const z: boolean = true;
  `,

  interface: `
    interface User {
      id: number;
      name: string;
      email: string;
      createdAt: Date;
    }

    interface Post {
      id: number;
      title: string;
      content: string;
      author: User;
      tags: string[];
    }

    const user: User = {
      id: 1,
      name: "Alice",
      email: "alice@example.com",
      createdAt: new Date(),
    };

    const post: Post = {
      id: 1,
      title: "Hello World",
      content: "This is my first post",
      author: user,
      tags: ["intro", "hello"],
    };
  `,

  generics: `
    function identity<T>(value: T): T {
      return value;
    }

    function map<T, U>(arr: T[], fn: (item: T) => U): U[] {
      return arr.map(fn);
    }

    interface Result<T, E> {
      ok: boolean;
      value?: T;
      error?: E;
    }

    function ok<T>(value: T): Result<T, never> {
      return { ok: true, value };
    }

    function err<E>(error: E): Result<never, E> {
      return { ok: false, error };
    }

    const nums = [1, 2, 3, 4, 5];
    const doubled = map(nums, n => n * 2);
    const result: Result<number, string> = ok(42);
  `,

  react: `
    import React, { useState, useEffect, useCallback, useMemo } from 'react';

    interface Todo {
      id: number;
      text: string;
      completed: boolean;
    }

    interface TodoItemProps {
      todo: Todo;
      onToggle: (id: number) => void;
      onDelete: (id: number) => void;
    }

    const TodoItem: React.FC<TodoItemProps> = ({ todo, onToggle, onDelete }) => {
      const handleToggle = useCallback(() => onToggle(todo.id), [todo.id, onToggle]);
      const handleDelete = useCallback(() => onDelete(todo.id), [todo.id, onDelete]);

      return (
        <li>
          <input type="checkbox" checked={todo.completed} onChange={handleToggle} />
          <span style={{ textDecoration: todo.completed ? 'line-through' : 'none' }}>
            {todo.text}
          </span>
          <button onClick={handleDelete}>Delete</button>
        </li>
      );
    };

    const TodoApp: React.FC = () => {
      const [todos, setTodos] = useState<Todo[]>([]);
      const [input, setInput] = useState('');

      const completedCount = useMemo(
        () => todos.filter(t => t.completed).length,
        [todos]
      );

      const addTodo = useCallback(() => {
        if (input.trim()) {
          setTodos(prev => [...prev, { id: Date.now(), text: input, completed: false }]);
          setInput('');
        }
      }, [input]);

      const toggleTodo = useCallback((id: number) => {
        setTodos(prev => prev.map(t => t.id === id ? { ...t, completed: !t.completed } : t));
      }, []);

      const deleteTodo = useCallback((id: number) => {
        setTodos(prev => prev.filter(t => t.id !== id));
      }, []);

      return (
        <div>
          <h1>Todo App ({completedCount}/{todos.length} completed)</h1>
          <input value={input} onChange={e => setInput(e.target.value)} />
          <button onClick={addTodo}>Add</button>
          <ul>
            {todos.map(todo => (
              <TodoItem key={todo.id} todo={todo} onToggle={toggleTodo} onDelete={deleteTodo} />
            ))}
          </ul>
        </div>
      );
    };

    export default TodoApp;
  `,

  complex: `
    // Complex TypeScript with advanced features
    type DeepPartial<T> = {
      [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
    };

    type DeepReadonly<T> = {
      readonly [P in keyof T]: T[P] extends object ? DeepReadonly<T[P]> : T[P];
    };

    interface ApiResponse<T> {
      data: T;
      status: number;
      headers: Record<string, string>;
      timestamp: Date;
    }

    interface PaginatedResponse<T> extends ApiResponse<T[]> {
      page: number;
      pageSize: number;
      total: number;
      hasMore: boolean;
    }

    class HttpClient {
      private baseUrl: string;

      constructor(baseUrl: string) {
        this.baseUrl = baseUrl;
      }

      async get<T>(path: string): Promise<ApiResponse<T>> {
        const response = await fetch(this.baseUrl + path);
        const data = await response.json();
        return {
          data,
          status: response.status,
          headers: Object.fromEntries(response.headers.entries()),
          timestamp: new Date(),
        };
      }

      async getPaginated<T>(path: string, page: number): Promise<PaginatedResponse<T>> {
        const response = await this.get<{ items: T[]; total: number }>(
          path + "?page=" + page
        );
        return {
          ...response,
          data: response.data.items,
          page,
          pageSize: 20,
          total: response.data.total,
          hasMore: page * 20 < response.data.total,
        };
      }
    }

    const client = new HttpClient("https://api.example.com");

    interface User {
      id: number;
      name: string;
      email: string;
    }

    async function fetchUsers(): Promise<User[]> {
      const response = await client.getPaginated<User>("/users", 1);
      return response.data;
    }
  `,
};

// Benchmark runner
interface BenchResult {
  name: string;
  iterations: number;
  totalMs: number;
  avgMs: number;
  minMs: number;
  maxMs: number;
  opsPerSec: number;
}

async function bench(name: string, fn: () => void | Promise<void>, iterations = 100): Promise<BenchResult> {
  // Warmup
  for (let i = 0; i < 5; i++) await fn();

  const times: number[] = [];
  const start = performance.now();

  for (let i = 0; i < iterations; i++) {
    const iterStart = performance.now();
    await fn();
    times.push(performance.now() - iterStart);
  }

  const totalMs = performance.now() - start;
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

function formatResult(r: BenchResult): string {
  return [
    `${r.name}:`,
    `  avg: ${r.avgMs.toFixed(2)}ms`,
    `  min: ${r.minMs.toFixed(2)}ms`,
    `  max: ${r.maxMs.toFixed(2)}ms`,
    `  ops/sec: ${r.opsPerSec.toFixed(1)}`,
  ].join("\n");
}

// Run benchmarks
console.log("=== tsgo Type Checker Benchmarks ===\n");
console.log(`Version: ${tsgo.version()}`);
console.log(`Platform: ${process.platform}-${process.arch}`);
console.log("");

const results: BenchResult[] = [];

// Single file benchmarks
console.log("--- Single File Type Checking ---\n");

results.push(await bench("simple (3 vars)", () => {
  tsgo.typecheck(samples.simple, "simple.ts");
}, 100));
console.log(formatResult(results[results.length - 1]));
console.log("");

results.push(await bench("interface (2 interfaces)", () => {
  tsgo.typecheck(samples.interface, "interface.ts");
}, 100));
console.log(formatResult(results[results.length - 1]));
console.log("");

results.push(await bench("generics (Result type)", () => {
  tsgo.typecheck(samples.generics, "generics.ts");
}, 100));
console.log(formatResult(results[results.length - 1]));
console.log("");

results.push(await bench("react (Todo app)", () => {
  tsgo.typecheck(samples.react, "react.tsx");
}, 50));
console.log(formatResult(results[results.length - 1]));
console.log("");

results.push(await bench("complex (HTTP client)", () => {
  tsgo.typecheck(samples.complex, "complex.ts");
}, 50));
console.log(formatResult(results[results.length - 1]));
console.log("");

// Multi-file benchmark
console.log("--- Multi-File Type Checking ---\n");

const multiFileProject = {
  "types.ts": `
    export interface User { id: number; name: string; email: string; }
    export interface Post { id: number; title: string; author: User; }
  `,
  "api.ts": `
    import { User, Post } from './types';
    export async function fetchUser(id: number): Promise<User> {
      return { id, name: 'User ' + id, email: 'user@example.com' };
    }
    export async function fetchPosts(userId: number): Promise<Post[]> {
      const user = await fetchUser(userId);
      return [{ id: 1, title: 'Post', author: user }];
    }
  `,
  "App.tsx": `
    import React, { useState, useEffect } from 'react';
    import { User, Post } from './types';
    import { fetchUser, fetchPosts } from './api';

    const App: React.FC = () => {
      const [user, setUser] = useState<User | null>(null);
      const [posts, setPosts] = useState<Post[]>([]);

      useEffect(() => {
        fetchUser(1).then(setUser);
        fetchPosts(1).then(setPosts);
      }, []);

      return (
        <div>
          <h1>{user?.name}</h1>
          <ul>{posts.map(p => <li key={p.id}>{p.title}</li>)}</ul>
        </div>
      );
    };
    export default App;
  `,
};

results.push(await bench("multi-file (3 files)", () => {
  tsgo.typecheckMultiple(multiFileProject, { jsx: "react-jsx", strict: true });
}, 50));
console.log(formatResult(results[results.length - 1]));
console.log("");

// Bundling benchmark (Bun.build)
console.log("--- Bundling with Bun.build ---\n");

// Create temp files in a place where node_modules can be found
const benchDir = import.meta.dir;
const tempDir = `${benchDir}/.temp`;
await Bun.$`mkdir -p ${tempDir}`;
await Bun.write(`${tempDir}/index.tsx`, samples.react);

results.push(await bench("bundle react app", async () => {
  await Bun.build({
    entrypoints: [`${tempDir}/index.tsx`],
    outdir: `${tempDir}/out`,
    minify: true,
    external: [], // Bundle everything
  });
}, 20));
console.log(formatResult(results[results.length - 1]));
console.log("");

// Combined pipeline
console.log("--- Full Pipeline (typecheck + bundle) ---\n");

results.push(await bench("typecheck + bundle", async () => {
  // Type check
  tsgo.typecheck(samples.react, "App.tsx");
  // Bundle
  await Bun.build({
    entrypoints: [`${tempDir}/index.tsx`],
    outdir: `${tempDir}/out`,
    minify: true,
  });
}, 20));
console.log(formatResult(results[results.length - 1]));
console.log("");

// Cleanup
await Bun.$`rm -rf ${tempDir}`.quiet();

// Summary
console.log("=== Summary ===\n");
console.log("| Benchmark | Avg (ms) | Ops/sec |");
console.log("|-----------|----------|---------|");
for (const r of results) {
  console.log(`| ${r.name.padEnd(25)} | ${r.avgMs.toFixed(2).padStart(8)} | ${r.opsPerSec.toFixed(1).padStart(7)} |`);
}
