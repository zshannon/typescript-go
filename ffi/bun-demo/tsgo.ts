/**
 * Bun FFI wrapper for tsgo (TypeScript-Go type checker)
 *
 * This module provides a high-performance TypeScript type checker
 * by wrapping the Go-based tsgo library via Bun's FFI.
 */

import { dlopen, FFIType, suffix, ptr, CString } from "bun:ffi";
import { resolve, dirname } from "path";

// Determine library path - look for libtsgo.so in parent directory
const libPath = resolve(dirname(import.meta.path), "..", `libtsgo.so`);

// Load the shared library
const lib = dlopen(libPath, {
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
  tsgo_free_string: {
    args: [FFIType.ptr],
    returns: FFIType.void,
  },
  tsgo_version: {
    args: [],
    returns: FFIType.ptr,
  },
});

// Get the directory where the wrapper is located (for finding node_modules)
const wrapperDir = dirname(import.meta.path);

// Helper to convert string to null-terminated buffer
function toCString(str: string): Buffer {
  return Buffer.from(str + "\0", "utf8");
}

// Helper to read C string from pointer and free it
function readAndFreeString(pointer: number | bigint): string {
  if (!pointer || pointer === 0n || pointer === 0) {
    return "";
  }
  const cstr = new CString(pointer);
  const result = cstr.toString();
  lib.symbols.tsgo_free_string(pointer);
  return result;
}

/** Diagnostic information from type checking */
export interface Diagnostic {
  code: number;
  category: string;
  message: string;
  file?: string;
  line?: number;
  column?: number;
  length?: number;
}

/** Result of type checking */
export interface TypeCheckResult {
  success: boolean;
  diagnostics: Diagnostic[];
  duration_ms: number;
}

/** Compiler options for type checking */
export interface CompilerOptions {
  target?: "ES5" | "ES6" | "ES2015" | "ES2016" | "ES2017" | "ES2018" | "ES2019" | "ES2020" | "ES2021" | "ES2022" | "ESNext";
  strict?: boolean;
  noEmit?: boolean;
  skipLibCheck?: boolean;
  jsx?: "react" | "react-jsx" | "react-jsxdev" | "preserve";
  lib?: string[];
}

/**
 * Type check TypeScript code
 * @param code - The TypeScript code to check
 * @param fileName - Optional file name (defaults to /project/input.tsx)
 * @param projectDir - Optional project directory containing node_modules (defaults to wrapper directory)
 * @returns Type check result with diagnostics
 */
export function typecheck(code: string, fileName?: string, projectDir?: string): TypeCheckResult {
  const codeBuffer = toCString(code);
  const fileNameBuffer = toCString(fileName || "/project/input.tsx");
  const projectDirBuffer = toCString(projectDir || wrapperDir);

  const resultPtr = lib.symbols.tsgo_typecheck(
    ptr(codeBuffer),
    ptr(fileNameBuffer),
    ptr(projectDirBuffer)
  );

  const jsonResult = readAndFreeString(resultPtr);
  return JSON.parse(jsonResult);
}

/**
 * Type check TypeScript code with custom compiler options
 * @param code - The TypeScript code to check
 * @param fileName - Optional file name
 * @param options - Compiler options
 * @param projectDir - Optional project directory containing node_modules
 * @returns Type check result with diagnostics
 */
export function typecheckWithOptions(
  code: string,
  fileName: string | undefined,
  options: CompilerOptions,
  projectDir?: string
): TypeCheckResult {
  const codeBuffer = toCString(code);
  const fileNameBuffer = toCString(fileName || "/project/input.tsx");
  const optionsBuffer = toCString(JSON.stringify(options));
  const projectDirBuffer = toCString(projectDir || wrapperDir);

  const resultPtr = lib.symbols.tsgo_typecheck_with_options(
    ptr(codeBuffer),
    ptr(fileNameBuffer),
    ptr(optionsBuffer),
    ptr(projectDirBuffer)
  );

  const jsonResult = readAndFreeString(resultPtr);
  return JSON.parse(jsonResult);
}

/**
 * Type check multiple files together
 * @param files - Map of file paths to their contents
 * @param options - Optional compiler options
 * @param projectDir - Optional project directory containing node_modules
 * @returns Type check result with diagnostics
 */
export function typecheckMultiple(
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

  const jsonResult = readAndFreeString(resultPtr);
  return JSON.parse(jsonResult);
}

/**
 * Get the version of the tsgo library
 * @returns Version string
 */
export function version(): string {
  const resultPtr = lib.symbols.tsgo_version();
  return readAndFreeString(resultPtr);
}

// Default export for convenience
export default {
  typecheck,
  typecheckWithOptions,
  typecheckMultiple,
  version,
};
