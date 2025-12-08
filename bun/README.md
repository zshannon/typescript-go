# @flickfyi/tsgo

Blazing fast TypeScript type checker powered by [TypeScript-Go](https://github.com/anthropics/typescript-go), accessible via Bun FFI.

**10-100x faster** than `tsc` for type checking operations.

## Features

- **Fast**: Native Go implementation, ~70ms for typical React components
- **Full TypeScript Support**: Generics, interfaces, classes, decorators, and more
- **JSX/TSX**: React, Preact, custom JSX runtimes via `jsxImportSource`
- **Multi-file Projects**: Cross-file imports and type resolution
- **Modern Defaults**: Strict mode, ES2022, Bundler module resolution
- **Rich Diagnostics**: Line numbers, columns, error codes, categories

## Installation

```bash
bun add @flickfyi/tsgo
```

The package automatically installs the correct binary for your platform:

| Platform | Architecture | Package |
|----------|--------------|---------|
| macOS | Apple Silicon (M1/M2/M3) | `@flickfyi/tsgo-darwin-arm64` |
| macOS | Intel | `@flickfyi/tsgo-darwin-x64` |
| Linux | x64 | `@flickfyi/tsgo-linux-x64` |
| Linux | ARM64 | `@flickfyi/tsgo-linux-arm64` |
| Windows | x64 | `@flickfyi/tsgo-win32-x64` |

## Quick Start

```typescript
import tsgo from '@flickfyi/tsgo';

// Simple type check
const result = tsgo.typecheck(`
  const x: number = 42;
  const y: string = "hello";
`, '/project/file.ts');

console.log(result.success); // true
console.log(result.duration_ms); // ~15ms
```

## API Reference

### `typecheck(code, fileName)`

Type check a single TypeScript/TSX file with sensible defaults.

```typescript
const result = tsgo.typecheck(code: string, fileName: string): TypeCheckResult;
```

**Parameters:**
- `code` - TypeScript source code
- `fileName` - Virtual file path (affects JSX detection: `.tsx` enables JSX)

**Returns:** `TypeCheckResult`

**Example:**
```typescript
// Catches type errors
const result = tsgo.typecheck(`
  const x: number = "not a number";
`, '/project/error.ts');

console.log(result.success); // false
console.log(result.diagnostics[0].message);
// "Type 'string' is not assignable to type 'number'"
console.log(result.diagnostics[0].code); // 2322
console.log(result.diagnostics[0].line); // 2
```

### `typecheckWithOptions(code, fileName, options)`

Type check with custom compiler options.

```typescript
const result = tsgo.typecheckWithOptions(
  code: string,
  fileName: string,
  options: CompilerOptions
): TypeCheckResult;
```

**Example - Custom JSX Runtime:**
```typescript
const result = tsgo.typecheckWithOptions(`
  const App = () => (
    <div css={{ color: 'blue' }}>
      Styled with Emotion
    </div>
  );
`, '/project/App.tsx', {
  jsx: 'react-jsx',
  jsxImportSource: '@emotion/react',
  strict: true,
  target: 'ES2022',
});
```

**Example - Legacy React:**
```typescript
const result = tsgo.typecheckWithOptions(code, '/project/App.tsx', {
  jsx: 'react',
  jsxFactory: 'React.createElement',
  jsxFragmentFactory: 'React.Fragment',
});
```

### `typecheckMultiple(files, options?)`

Type check multiple files as a project with cross-file imports.

```typescript
const result = tsgo.typecheckMultiple(
  files: Record<string, string>,
  options?: CompilerOptions
): TypeCheckResult;
```

**Example:**
```typescript
const result = tsgo.typecheckMultiple({
  '/project/types.ts': `
    export interface User {
      id: number;
      name: string;
      email: string;
    }
  `,
  '/project/api.ts': `
    import { User } from './types';

    export async function fetchUser(id: number): Promise<User> {
      const response = await fetch(\`/api/users/\${id}\`);
      return response.json();
    }
  `,
  '/project/App.tsx': `
    import React, { useState, useEffect } from 'react';
    import { User } from './types';
    import { fetchUser } from './api';

    const UserCard: React.FC<{ userId: number }> = ({ userId }) => {
      const [user, setUser] = useState<User | null>(null);

      useEffect(() => {
        fetchUser(userId).then(setUser);
      }, [userId]);

      if (!user) return <div>Loading...</div>;

      return (
        <div>
          <h1>{user.name}</h1>
          <p>{user.email}</p>
        </div>
      );
    };

    export default UserCard;
  `,
}, {
  jsx: 'react-jsx',
  strict: true,
  target: 'ES2022',
});
```

### `version()`

Get the library version.

```typescript
console.log(tsgo.version()); // "1.0.0"
```

## Types

### `CompilerOptions`

```typescript
interface CompilerOptions {
  // Target & Module
  target?: "ES5" | "ES6" | "ES2015" | ... | "ES2022" | "ESNext";
  module?: "CommonJS" | "ESNext" | "Node16" | "NodeNext" | "Preserve";
  moduleResolution?: "Classic" | "Node" | "Node10" | "Node16" | "NodeNext" | "Bundler";
  lib?: string[];

  // JSX
  jsx?: "react" | "react-jsx" | "react-jsxdev" | "preserve";
  jsxImportSource?: string;  // e.g., "react", "@emotion/react", "preact"
  jsxFactory?: string;       // e.g., "React.createElement"
  jsxFragmentFactory?: string;

  // Strict Type Checking
  strict?: boolean;
  strictNullChecks?: boolean;

  // Module Interop
  esModuleInterop?: boolean;
  resolveJsonModule?: boolean;
  isolatedModules?: boolean;

  // Output
  noEmit?: boolean;
  declaration?: boolean;

  // Other
  allowJs?: boolean;
  skipLibCheck?: boolean;
  forceConsistentCasingInFileNames?: boolean;
}
```

### `TypeCheckResult`

```typescript
interface TypeCheckResult {
  success: boolean;          // true if no errors
  diagnostics: Diagnostic[]; // Array of issues found
  duration_ms: number;       // Time taken in milliseconds
}
```

### `Diagnostic`

```typescript
interface Diagnostic {
  code: number;              // TypeScript error code (e.g., 2322)
  category: "error" | "warning" | "suggestion" | "message";
  message: string;           // Human-readable message
  file?: string;             // Source file (for multi-file)
  line?: number;             // 1-based line number
  column?: number;           // 1-based column number
}
```

## Default Options

When using `typecheck()` without explicit options, these defaults apply:

```typescript
{
  strict: true,
  strictNullChecks: true,
  target: "ES2022",
  module: "ESNext",
  moduleResolution: "Bundler",
  jsx: "react-jsx",
  esModuleInterop: true,
  isolatedModules: true,
  resolveJsonModule: true,
  skipLibCheck: true,
  noEmit: true,
  allowJs: true,
  declaration: true,
  forceConsistentCasingInFileNames: true,
  lib: ["ES2022", "DOM"],
}
```

## Performance

Benchmarks on Apple M1 MacBook Pro:

| Operation | Time |
|-----------|------|
| Simple TypeScript | ~15ms |
| React component | ~70ms |
| Multi-file project (3 files) | ~85ms |
| Complex generics | ~25ms |

Compared to `tsc --noEmit`:
- **10-100x faster** for single files
- **5-20x faster** for small projects

## Use Cases

### IDE/Editor Integration
```typescript
// Real-time type checking as users type
const checkTypes = debounce((code: string) => {
  const result = tsgo.typecheck(code, '/project/current.tsx');
  updateDiagnostics(result.diagnostics);
}, 100);
```

### CI/Build Pipeline
```typescript
// Fast type checking in CI
const files = await glob('src/**/*.{ts,tsx}');
const fileMap = Object.fromEntries(
  await Promise.all(files.map(async f => [f, await Bun.file(f).text()]))
);

const result = tsgo.typecheckMultiple(fileMap, { strict: true });
if (!result.success) {
  console.error('Type errors found:');
  for (const d of result.diagnostics) {
    console.error(`${d.file}:${d.line}:${d.column} - ${d.message}`);
  }
  process.exit(1);
}
```

### Custom JSX Frameworks
```typescript
// Validate custom JSX elements
const result = tsgo.typecheckWithOptions(code, '/project/App.tsx', {
  jsx: 'react-jsx',
  jsxImportSource: '@my-framework/jsx-runtime',
});
```

## Requirements

- **Bun** >= 1.0.0 (uses Bun FFI)
- One of the supported platforms (see Installation)

## License

Apache-2.0

## Contributing

See [CONTRIBUTING.md](https://github.com/anthropics/typescript-go/blob/main/CONTRIBUTING.md) for development setup.

## Related

- [TypeScript-Go](https://github.com/anthropics/typescript-go) - The Go implementation this package wraps
- [Bun](https://bun.sh) - The JavaScript runtime this package is built for
