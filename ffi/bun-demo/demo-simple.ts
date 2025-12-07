#!/usr/bin/env bun
/**
 * Simple Demo: Pure TypeScript type checking without external dependencies
 */

import tsgo from "./tsgo";

console.log("=".repeat(60));
console.log("tsgo Bun FFI Demo - Pure TypeScript Type Checking");
console.log("=".repeat(60));
console.log(`\ntsgo version: ${tsgo.version()}\n`);

// Valid TypeScript code
const validCode = `
interface User {
  id: number;
  name: string;
  email: string;
  active: boolean;
}

function createUser(name: string, email: string): User {
  return {
    id: Math.random() * 1000,
    name,
    email,
    active: true,
  };
}

function getUserDisplayName(user: User): string {
  return user.active ? user.name : \`[Inactive] \${user.name}\`;
}

const user1 = createUser("Alice", "alice@example.com");
const displayName = getUserDisplayName(user1);
console.log(displayName);

// Generic function
function first<T>(arr: T[]): T | undefined {
  return arr.length > 0 ? arr[0] : undefined;
}

const numbers = [1, 2, 3];
const firstNum = first(numbers); // Type: number | undefined

// Class example
class Counter {
  private count: number = 0;

  increment(): void {
    this.count++;
  }

  decrement(): void {
    this.count--;
  }

  getCount(): number {
    return this.count;
  }
}

const counter = new Counter();
counter.increment();
counter.increment();
console.log(counter.getCount()); // 2
`;

console.log("1. Type checking valid TypeScript code...\n");
console.log("Code:");
console.log("-".repeat(40));
console.log(validCode.trim());
console.log("-".repeat(40));

const validResult = tsgo.typecheck(validCode, "/project/app.ts");

console.log("\nResult:");
console.log(`  Success: ${validResult.success}`);
console.log(`  Duration: ${validResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${validResult.diagnostics?.length ?? 0}`);

// Code with type errors
const codeWithErrors = `
interface Product {
  id: number;
  name: string;
  price: number;
}

function calculateDiscount(product: Product, percentage: number): number {
  return product.price * (percentage / 100);
}

// Error 1: Wrong argument type
const discount1 = calculateDiscount("invalid", 10);

// Error 2: Missing property
const product: Product = {
  id: 1,
  name: "Test",
  // missing price
};

// Error 3: Wrong return type
function getProductName(p: Product): number {
  return p.name; // returning string instead of number
}

// Error 4: Calling method on possibly undefined
const items: string[] = [];
console.log(items[0].toUpperCase());
`;

console.log("\n" + "=".repeat(60));
console.log("2. Type checking code with errors...\n");
console.log("Code:");
console.log("-".repeat(40));
console.log(codeWithErrors.trim());
console.log("-".repeat(40));

const errorResult = tsgo.typecheck(codeWithErrors, "/project/errors.ts");

console.log("\nResult:");
console.log(`  Success: ${errorResult.success}`);
console.log(`  Duration: ${errorResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${errorResult.diagnostics?.length ?? 0}`);

if (errorResult.diagnostics?.length ?? 0 > 0) {
  console.log("\n  Detected Errors:");
  for (const diag of errorResult.diagnostics) {
    console.log(`\n    [${diag.code}] ${diag.message}`);
    if (diag.line) {
      console.log(`    Location: line ${diag.line}, column ${diag.column}`);
    }
  }
}

// Multi-file project
console.log("\n" + "=".repeat(60));
console.log("3. Multi-file project type checking...\n");

const projectFiles = {
  "/project/models/user.ts": `
export interface User {
  id: number;
  name: string;
  email: string;
}

export interface Admin extends User {
  permissions: string[];
}
`,
  "/project/services/userService.ts": `
import { User, Admin } from '../models/user';

export class UserService {
  private users: Map<number, User> = new Map();

  addUser(user: User): void {
    this.users.set(user.id, user);
  }

  getUser(id: number): User | undefined {
    return this.users.get(id);
  }

  isAdmin(user: User): user is Admin {
    return 'permissions' in user;
  }
}
`,
  "/project/main.ts": `
import { User, Admin } from './models/user';
import { UserService } from './services/userService';

const service = new UserService();

const regularUser: User = {
  id: 1,
  name: "Alice",
  email: "alice@example.com"
};

const adminUser: Admin = {
  id: 2,
  name: "Bob",
  email: "bob@example.com",
  permissions: ["read", "write", "delete"]
};

service.addUser(regularUser);
service.addUser(adminUser);

const found = service.getUser(1);
if (found && service.isAdmin(found)) {
  console.log(found.permissions);
}
`,
};

console.log("Files:");
for (const [path, content] of Object.entries(projectFiles)) {
  const lines = content.trim().split("\n").length;
  console.log(`  ${path} (${lines} lines)`);
}

const multiResult = tsgo.typecheckMultiple(projectFiles, {
  strict: true,
  target: "ES2022",
});

console.log("\nResult:");
console.log(`  Success: ${multiResult.success}`);
console.log(`  Duration: ${multiResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${multiResult.diagnostics?.length ?? 0}`);

if (multiResult.diagnostics?.length ?? 0 > 0) {
  console.log("\n  Errors:");
  for (const diag of multiResult.diagnostics) {
    console.log(`    - ${diag.message}`);
    if (diag.file) {
      console.log(`      in ${diag.file}:${diag.line}:${diag.column}`);
    }
  }
} else {
  console.log("\n  All files type-check successfully!");
}

console.log("\n" + "=".repeat(60));
console.log("Demo complete!");
console.log("=".repeat(60));
