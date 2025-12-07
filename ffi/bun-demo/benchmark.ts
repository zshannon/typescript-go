#!/usr/bin/env bun
/**
 * Benchmark: Compare tsgo FFI type checking performance
 */

import tsgo from "./tsgo";

console.log("=".repeat(60));
console.log("tsgo Bun FFI Benchmark");
console.log("=".repeat(60));
console.log(`\ntsgo version: ${tsgo.version()}\n`);

// Sample code snippets of varying complexity
const samples = {
  simple: `
const x: number = 42;
const y: string = "hello";
const z: boolean = true;
`,

  medium: `
interface User {
  id: number;
  name: string;
  email: string;
  roles: string[];
}

function createUser(name: string, email: string): User {
  return {
    id: Math.random(),
    name,
    email,
    roles: ['user'],
  };
}

const users: User[] = [
  createUser('Alice', 'alice@example.com'),
  createUser('Bob', 'bob@example.com'),
];

function findUser(id: number): User | undefined {
  return users.find(u => u.id === id);
}
`,

  complex: `
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
`,
};

// Benchmark function
function benchmark(
  name: string,
  fn: () => void,
  iterations: number = 100
): { name: string; iterations: number; totalMs: number; avgMs: number; opsPerSec: number } {
  // Warmup
  for (let i = 0; i < 5; i++) {
    fn();
  }

  // Actual benchmark
  const start = performance.now();
  for (let i = 0; i < iterations; i++) {
    fn();
  }
  const totalMs = performance.now() - start;
  const avgMs = totalMs / iterations;
  const opsPerSec = 1000 / avgMs;

  return { name, iterations, totalMs, avgMs, opsPerSec };
}

console.log("Running benchmarks...\n");

const results: Array<ReturnType<typeof benchmark>> = [];

// Benchmark each sample
for (const [name, code] of Object.entries(samples)) {
  const iterations = name === "complex" ? 20 : 50;
  const result = benchmark(
    `typecheck-${name}`,
    () => tsgo.typecheck(code, `/project/${name}.tsx`),
    iterations
  );
  results.push(result);
  console.log(
    `${result.name}: ${result.avgMs.toFixed(2)}ms avg (${result.opsPerSec.toFixed(1)} ops/sec)`
  );
}

// Benchmark with options
console.log("\n--- With custom options ---");
const optionsResult = benchmark(
  "typecheck-with-options",
  () =>
    tsgo.typecheckWithOptions(samples.medium, "/project/test.ts", {
      strict: true,
      target: "ES2022",
      skipLibCheck: true,
    }),
  50
);
results.push(optionsResult);
console.log(
  `${optionsResult.name}: ${optionsResult.avgMs.toFixed(2)}ms avg (${optionsResult.opsPerSec.toFixed(1)} ops/sec)`
);

// Benchmark multi-file
console.log("\n--- Multi-file type checking ---");
const multiFiles = {
  "/project/types.ts": samples.simple,
  "/project/utils.ts": samples.medium,
  "/project/App.tsx": samples.complex,
};

const multiResult = benchmark(
  "typecheck-multiple",
  () => tsgo.typecheckMultiple(multiFiles),
  20
);
results.push(multiResult);
console.log(
  `${multiResult.name}: ${multiResult.avgMs.toFixed(2)}ms avg (${multiResult.opsPerSec.toFixed(1)} ops/sec)`
);

// Summary
console.log("\n" + "=".repeat(60));
console.log("Summary");
console.log("=".repeat(60));
console.log("\n| Benchmark | Avg (ms) | Ops/sec | Iterations |");
console.log("|-----------|----------|---------|------------|");
for (const r of results) {
  console.log(
    `| ${r.name.padEnd(25)} | ${r.avgMs.toFixed(2).padStart(8)} | ${r.opsPerSec.toFixed(1).padStart(7)} | ${r.iterations.toString().padStart(10)} |`
  );
}

// Code size stats
console.log("\n" + "=".repeat(60));
console.log("Code Size Statistics");
console.log("=".repeat(60));
for (const [name, code] of Object.entries(samples)) {
  const lines = code.trim().split("\n").length;
  const chars = code.length;
  console.log(`${name}: ${lines} lines, ${chars} characters`);
}

console.log("\n" + "=".repeat(60));
console.log("Benchmark complete!");
console.log("=".repeat(60));
