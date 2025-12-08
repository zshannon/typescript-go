/**
 * @module tsgo
 * @description Blazing fast TypeScript type checker powered by TypeScript-Go
 *
 * @example
 * ```typescript
 * import tsgo from 'tsgo';
 *
 * // Simple type check
 * const result = tsgo.typecheck(`const x: number = 42;`, '/project/file.ts');
 * console.log(result.success); // true
 *
 * // Type check with custom options
 * const result = tsgo.typecheckWithOptions(code, '/project/App.tsx', {
 *   jsx: 'react-jsx',
 *   strict: true,
 *   target: 'ES2022',
 * });
 *
 * // Multi-file project
 * const result = tsgo.typecheckMultiple({
 *   '/project/types.ts': 'export interface User { name: string; }',
 *   '/project/main.ts': 'import { User } from "./types"; const u: User = { name: "Alice" };',
 * }, { strict: true });
 * ```
 */

import { dlopen, FFIType, ptr, CString } from "bun:ffi";
import { existsSync } from "fs";
import { join, dirname } from "path";
import { platform, arch } from "os";

// ============================================================================
// Types
// ============================================================================

/**
 * TypeScript compiler options
 * @see https://www.typescriptlang.org/tsconfig
 */
export interface CompilerOptions {
  // Target & Module
  /** ECMAScript target version */
  target?: "ES5" | "ES6" | "ES2015" | "ES2016" | "ES2017" | "ES2018" | "ES2019" | "ES2020" | "ES2021" | "ES2022" | "ESNext";
  /** Module system to use */
  module?: "CommonJS" | "AMD" | "UMD" | "System" | "ES6" | "ES2015" | "ES2020" | "ES2022" | "ESNext" | "Node16" | "NodeNext" | "Preserve";
  /** Module resolution strategy */
  moduleResolution?: "Classic" | "Node" | "Node10" | "Node16" | "NodeNext" | "Bundler";
  /** Library files to include */
  lib?: string[];

  // JSX Options
  /** JSX emit mode */
  jsx?: "react" | "react-jsx" | "react-jsxdev" | "preserve";
  /** Module specifier for importing JSX factory functions (e.g., "react", "@emotion/react") */
  jsxImportSource?: string;
  /** JSX factory function (e.g., "React.createElement") */
  jsxFactory?: string;
  /** JSX fragment factory (e.g., "React.Fragment") */
  jsxFragmentFactory?: string;

  // Strict Type Checking
  /** Enable all strict type checking options */
  strict?: boolean;
  /** Enable strict null checks */
  strictNullChecks?: boolean;

  // Module Interop
  /** Enable ES module interop */
  esModuleInterop?: boolean;
  /** Allow importing JSON modules */
  resolveJsonModule?: boolean;
  /** Ensure each file can be safely transpiled */
  isolatedModules?: boolean;

  // Output
  /** Do not emit output files */
  noEmit?: boolean;
  /** Generate .d.ts declaration files */
  declaration?: boolean;

  // Other
  /** Allow JavaScript files */
  allowJs?: boolean;
  /** Skip type checking of declaration files */
  skipLibCheck?: boolean;
  /** Enforce consistent casing in file names */
  forceConsistentCasingInFileNames?: boolean;
}

/**
 * Diagnostic severity category
 */
export type DiagnosticCategory = "error" | "warning" | "suggestion" | "message";

/**
 * A diagnostic message from the type checker
 */
export interface Diagnostic {
  /** Error code (e.g., 2322 for type mismatch) */
  code: number;
  /** Diagnostic category */
  category: DiagnosticCategory;
  /** Human-readable error message */
  message: string;
  /** Source file path (for multi-file projects) */
  file?: string;
  /** 1-based line number */
  line?: number;
  /** 1-based column number */
  column?: number;
}

/**
 * Result of type checking operation
 */
export interface TypeCheckResult {
  /** Whether type checking passed with no errors */
  success: boolean;
  /** Array of diagnostic messages */
  diagnostics: Diagnostic[];
  /** Time taken in milliseconds */
  duration_ms: number;
}

// ============================================================================
// Binary Loading
// ============================================================================

/**
 * Get the path to the native binary for the current platform
 */
function getBinaryPath(): string {
  const currentPlatform = platform();
  const currentArch = arch();

  // Map Node.js platform/arch to our package names
  const platformMap: Record<string, string> = {
    darwin: "darwin",
    linux: "linux",
    win32: "win32",
  };

  const archMap: Record<string, string> = {
    arm64: "arm64",
    x64: "x64",
    x86_64: "x64",
  };

  const plat = platformMap[currentPlatform];
  const ar = archMap[currentArch];

  if (!plat || !ar) {
    throw new Error(
      `Unsupported platform: ${currentPlatform}-${currentArch}. ` +
      `Supported platforms: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64`
    );
  }

  const packageName = `@flickfyi/tsgo-${plat}-${ar}`;
  const extension = currentPlatform === "win32" ? ".dll" : currentPlatform === "darwin" ? ".dylib" : ".so";
  const binaryName = `libtsgo${extension}`;

  // Try to find the binary in various locations
  const paths = [
    // Development mode - binary in package root
    join(import.meta.dir, "..", binaryName),
    // Installed as dependency - platform package in node_modules
    join(dirname(import.meta.dir), "node_modules", packageName, binaryName),
    // Installed at project root
    join(process.cwd(), "node_modules", packageName, binaryName),
  ];

  for (const p of paths) {
    if (existsSync(p)) {
      return p;
    }
  }

  throw new Error(
    `Could not find tsgo binary for ${currentPlatform}-${currentArch}.\n` +
    `Tried paths:\n${paths.map(p => `  - ${p}`).join("\n")}\n\n` +
    `Please ensure the platform-specific package is installed:\n` +
    `  bun add @flickfyi/tsgo`
  );
}

// ============================================================================
// FFI Setup
// ============================================================================

const binaryPath = getBinaryPath();

const lib = dlopen(binaryPath, {
  tsgo_typecheck: {
    args: [FFIType.ptr, FFIType.ptr, FFIType.ptr],
    returns: FFIType.ptr,
  },
  tsgo_typecheck_with_options: {
    args: [FFIType.ptr, FFIType.ptr, FFIType.ptr, FFIType.ptr],
    returns: FFIType.ptr,
  },
  tsgo_typecheck_multiple: {
    args: [FFIType.ptr, FFIType.ptr, FFIType.ptr],
    returns: FFIType.ptr,
  },
  tsgo_version: {
    args: [],
    returns: FFIType.ptr,
  },
  tsgo_free_string: {
    args: [FFIType.ptr],
    returns: FFIType.void,
  },
});

// Get the wrapper directory for finding node_modules
// This should be the package root or the current working directory
const wrapperDir = process.cwd();

// ============================================================================
// Helper Functions
// ============================================================================

/** Convert string to null-terminated C string buffer */
function toCString(str: string): Buffer {
  return Buffer.from(str + "\0", "utf8");
}

/** Read C string from pointer and free it */
function readAndFreeString(pointer: number | bigint): string {
  if (!pointer || pointer === 0n || pointer === 0) {
    return "";
  }
  const cstr = new CString(pointer);
  const result = cstr.toString();
  lib.symbols.tsgo_free_string(pointer);
  return result;
}

function parseResult(resultPtr: number | bigint): TypeCheckResult {
  if (!resultPtr || resultPtr === 0n || resultPtr === 0) {
    return {
      success: false,
      diagnostics: [{ code: 0, category: "error", message: "FFI call returned null" }],
      duration_ms: 0,
    };
  }

  const resultStr = readAndFreeString(resultPtr);

  try {
    return JSON.parse(resultStr);
  } catch {
    return {
      success: false,
      diagnostics: [{ code: 0, category: "error", message: `Failed to parse result: ${resultStr}` }],
      duration_ms: 0,
    };
  }
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Get the version of the tsgo library
 *
 * @returns Version string (e.g., "1.0.0")
 *
 * @example
 * ```typescript
 * console.log(tsgo.version()); // "1.0.0"
 * ```
 */
function version(): string {
  const resultPtr = lib.symbols.tsgo_version();
  if (!resultPtr) return "unknown";
  const cstr = new CString(resultPtr);
  return cstr.toString();
}

/**
 * Type check a single TypeScript/TSX file
 *
 * Uses sensible defaults optimized for modern TypeScript development:
 * - strict: true
 * - jsx: react-jsx
 * - module: ESNext
 * - moduleResolution: Bundler
 * - target: ES2022
 *
 * @param code - The TypeScript source code to check
 * @param fileName - Virtual file path (used for error messages and JSX detection)
 * @returns Type checking result with diagnostics
 *
 * @example
 * ```typescript
 * // Basic TypeScript
 * const result = tsgo.typecheck(`const x: number = "wrong";`, '/project/file.ts');
 * // result.success === false
 * // result.diagnostics[0].message contains type error
 *
 * // React component
 * const result = tsgo.typecheck(`
 *   import React from 'react';
 *   const App: React.FC = () => <div>Hello</div>;
 * `, '/project/App.tsx');
 * ```
 */
function typecheck(code: string, fileName: string, projectDir?: string): TypeCheckResult {
  const codeBuffer = toCString(code);
  const fileNameBuffer = toCString(fileName);
  const projectDirBuffer = toCString(projectDir || wrapperDir);

  const resultPtr = lib.symbols.tsgo_typecheck(
    ptr(codeBuffer),
    ptr(fileNameBuffer),
    ptr(projectDirBuffer)
  );
  return parseResult(resultPtr);
}

/**
 * Type check a single file with custom compiler options
 *
 * @param code - The TypeScript source code to check
 * @param fileName - Virtual file path (used for error messages and JSX detection)
 * @param options - TypeScript compiler options
 * @returns Type checking result with diagnostics
 *
 * @example
 * ```typescript
 * // Custom JSX runtime (e.g., Emotion, Preact, custom)
 * const result = tsgo.typecheckWithOptions(code, '/project/App.tsx', {
 *   jsx: 'react-jsx',
 *   jsxImportSource: '@emotion/react',
 *   strict: true,
 * });
 *
 * // Legacy React
 * const result = tsgo.typecheckWithOptions(code, '/project/App.tsx', {
 *   jsx: 'react',
 *   jsxFactory: 'React.createElement',
 *   jsxFragmentFactory: 'React.Fragment',
 * });
 * ```
 */
function typecheckWithOptions(
  code: string,
  fileName: string,
  options: CompilerOptions,
  projectDir?: string
): TypeCheckResult {
  const codeBuffer = toCString(code);
  const fileNameBuffer = toCString(fileName);
  const optionsBuffer = toCString(JSON.stringify(options));
  const projectDirBuffer = toCString(projectDir || wrapperDir);

  const resultPtr = lib.symbols.tsgo_typecheck_with_options(
    ptr(codeBuffer),
    ptr(fileNameBuffer),
    ptr(optionsBuffer),
    ptr(projectDirBuffer)
  );
  return parseResult(resultPtr);
}

/**
 * Type check multiple files as a project
 *
 * Files can import from each other using relative paths.
 * The type checker resolves imports between virtual files.
 *
 * @param files - Map of file paths to source code
 * @param options - TypeScript compiler options (optional)
 * @returns Type checking result with diagnostics for all files
 *
 * @example
 * ```typescript
 * const result = tsgo.typecheckMultiple({
 *   '/project/types.ts': `
 *     export interface User {
 *       id: number;
 *       name: string;
 *     }
 *   `,
 *   '/project/utils.ts': `
 *     import { User } from './types';
 *     export function greet(user: User): string {
 *       return \`Hello, \${user.name}!\`;
 *     }
 *   `,
 *   '/project/main.ts': `
 *     import { User } from './types';
 *     import { greet } from './utils';
 *     const user: User = { id: 1, name: 'Alice' };
 *     console.log(greet(user));
 *   `,
 * }, {
 *   strict: true,
 *   target: 'ES2022',
 * });
 * ```
 */
function typecheckMultiple(
  files: Record<string, string>,
  options?: CompilerOptions,
  projectDir?: string
): TypeCheckResult {
  const filesBuffer = toCString(JSON.stringify(files));
  const optionsBuffer = toCString(options ? JSON.stringify(options) : "");
  const projectDirBuffer = toCString(projectDir || wrapperDir);

  const resultPtr = lib.symbols.tsgo_typecheck_multiple(
    ptr(filesBuffer),
    ptr(optionsBuffer),
    ptr(projectDirBuffer)
  );
  return parseResult(resultPtr);
}

// ============================================================================
// Default Export
// ============================================================================

const tsgo = {
  version,
  typecheck,
  typecheckWithOptions,
  typecheckMultiple,
};

export default tsgo;
export { version, typecheck, typecheckWithOptions, typecheckMultiple };
