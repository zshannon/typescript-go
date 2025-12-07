#!/usr/bin/env bun
/**
 * Demo: Type checking and bundling a React Hello World component
 * using tsgo via Bun FFI
 */

import tsgo from "./tsgo";

console.log("=".repeat(60));
console.log("tsgo Bun FFI Demo - React Hello World Type Checking");
console.log("=".repeat(60));
console.log(`\ntsgo version: ${tsgo.version()}\n`);

// React Hello World component
const helloWorldComponent = `
import React from 'react';

interface HelloProps {
  name: string;
  greeting?: string;
}

export const HelloWorld: React.FC<HelloProps> = ({ name, greeting = "Hello" }) => {
  return (
    <div className="hello-container">
      <h1>{greeting}, {name}!</h1>
      <p>Welcome to React with TypeScript-Go type checking.</p>
    </div>
  );
};

// Usage example
const App = () => {
  return (
    <div>
      <HelloWorld name="World" />
      <HelloWorld name="Bun" greeting="Greetings" />
    </div>
  );
};

export default App;
`;

console.log("1. Type checking a valid React component...\n");
console.log("Code:");
console.log("-".repeat(40));
console.log(helloWorldComponent.trim());
console.log("-".repeat(40));

const validResult = tsgo.typecheck(helloWorldComponent, "/project/HelloWorld.tsx");

console.log("\nResult:");
console.log(`  Success: ${validResult.success}`);
console.log(`  Duration: ${validResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${validResult.diagnostics?.length ?? 0}`);

if ((validResult.diagnostics?.length ?? 0) > 0) {
  console.log("\n  Errors:");
  for (const diag of validResult.diagnostics) {
    console.log(`    - [${diag.category}] ${diag.message}`);
    if (diag.line) {
      console.log(`      at line ${diag.line}, column ${diag.column}`);
    }
  }
}

// Component with type error
const componentWithError = `
import React from 'react';

interface ButtonProps {
  label: string;
  onClick: () => void;
  disabled?: boolean;
}

const Button: React.FC<ButtonProps> = ({ label, onClick, disabled }) => {
  return (
    <button onClick={onClick} disabled={disabled}>
      {label}
    </button>
  );
};

// Type error: missing required 'onClick' prop
const App = () => {
  return <Button label="Click me" />;
};

export default App;
`;

console.log("\n" + "=".repeat(60));
console.log("2. Type checking a component with errors...\n");
console.log("Code:");
console.log("-".repeat(40));
console.log(componentWithError.trim());
console.log("-".repeat(40));

const errorResult = tsgo.typecheck(componentWithError, "/project/Button.tsx");

console.log("\nResult:");
console.log(`  Success: ${errorResult.success}`);
console.log(`  Duration: ${errorResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${errorResult.diagnostics?.length ?? 0}`);

if ((errorResult.diagnostics?.length ?? 0) > 0) {
  console.log("\n  Errors:");
  for (const diag of errorResult.diagnostics) {
    console.log(`    - [${diag.category}] ${diag.message}`);
    if (diag.line) {
      console.log(`      at line ${diag.line}, column ${diag.column}`);
    }
  }
}

// Multi-file type checking
console.log("\n" + "=".repeat(60));
console.log("3. Multi-file type checking...\n");

const files = {
  "/project/types.ts": `
export interface User {
  id: number;
  name: string;
  email: string;
}

export type UserRole = 'admin' | 'user' | 'guest';
`,
  "/project/utils.ts": `
import { User, UserRole } from './types';

export function formatUser(user: User, role: UserRole): string {
  return \`[\${role}] \${user.name} <\${user.email}>\`;
}

export function validateEmail(email: string): boolean {
  return email.includes('@');
}
`,
  "/project/App.tsx": `
import React from 'react';
import { User } from './types';
import { formatUser } from './utils';

interface AppProps {
  currentUser: User;
}

const App: React.FC<AppProps> = ({ currentUser }) => {
  const formatted = formatUser(currentUser, 'admin');

  return (
    <div>
      <h1>User Dashboard</h1>
      <p>{formatted}</p>
    </div>
  );
};

export default App;
`,
};

console.log("Files:");
for (const [path, content] of Object.entries(files)) {
  console.log(`  ${path} (${content.length} chars)`);
}

const multiResult = tsgo.typecheckMultiple(files, {
  strict: true,
  jsx: "react-jsx",
});

console.log("\nResult:");
console.log(`  Success: ${multiResult.success}`);
console.log(`  Duration: ${multiResult.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${multiResult.diagnostics?.length ?? 0}`);

if ((multiResult.diagnostics?.length ?? 0) > 0) {
  console.log("\n  Errors:");
  for (const diag of multiResult.diagnostics) {
    console.log(`    - [${diag.category}] ${diag.message}`);
    if (diag.file) {
      console.log(`      in ${diag.file} at line ${diag.line}, column ${diag.column}`);
    }
  }
}

console.log("\n" + "=".repeat(60));
console.log("Demo complete!");
console.log("=".repeat(60));
