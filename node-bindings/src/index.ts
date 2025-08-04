import { createRequire } from 'module';
import { promisify } from 'util';
import type {
  BuildResult,
  BuildOptions,
  Source,
  FileResolver,
  FileResolverResult,
  TSConfig,
  CompilerOptions
} from './types.js';

import {
  ECMAScriptTarget,
  ModuleKind
} from './types.js';

// Load native binding
const require = createRequire(import.meta.url);
const binding = require('../build/Release/tscgo.node');

// Promisify the dynamic resolver function
const buildWithDynamicResolverAsync = promisify(binding.buildWithDynamicResolver);

/**
 * Build TypeScript files from the filesystem
 * @param projectPath Path to the TypeScript project directory or tsconfig.json file
 * @param options Build options
 * @returns Build result with compilation status and diagnostics
 */
export function buildFromFileSystem(
  projectPath: string,
  options: BuildOptions = {}
): BuildResult {
  const { printErrors = false, configFile = '' } = options;
  return binding.buildFileSystem(projectPath, printErrors, configFile);
}

/**
 * Build TypeScript files from in-memory sources
 * @param sourceFiles Array of source files with name and content
 * @param config TypeScript configuration (optional)
 * @returns Build result with compilation status and diagnostics
 */
export async function build(
  sourceFiles: Source[],
  config: TSConfig = defaultConfig
): Promise<BuildResult> {
  // Create a resolver that handles the source files
  const resolver: FileResolver = (path: string) => {
    if (path === '/project') {
      return { type: 'directory', files: ['src', 'tsconfig.json'] };
    }
    if (path === '/project/src') {
      return { type: 'directory', files: sourceFiles.map(f => f.name) };
    }
    
    const sourceFile = sourceFiles.find(f => `/project/src/${f.name}` === path);
    if (sourceFile) {
      return { type: 'file', content: sourceFile.content };
    }
    
    // Handle tsconfig.json
    if (path === '/project/tsconfig.json') {
      const configWithDefaults = { ...config };
      configWithDefaults.include = configWithDefaults.include || ['src/**/*'];
      return { type: 'file', content: JSON.stringify(configWithDefaults, null, 2) };
    }
    
    return null;
  };
  
  return buildWithResolver(resolver, config);
}

/**
 * Build TypeScript files with a custom file resolver
 * @param resolver Function that resolves file paths to content or directory listings
 * @param config TypeScript configuration (optional)
 * @returns Build result with compilation status and diagnostics
 */
export async function buildWithResolver(
  resolver: FileResolver,
  config: TSConfig = defaultConfig
): Promise<BuildResult> {
  const projectPath = '/project';
  
  // For buildWithDynamicResolver, we need to handle async resolvers differently
  // The C++ code expects a synchronous function, but our resolver might be async
  // We'll create a map to cache results for synchronous access
  const resolverCache = new Map<string, FileResolverResult>();
  let cacheInitialized = false;
  
  // Pre-populate cache with known paths
  const initializeCache = async () => {
    if (cacheInitialized) return;
    cacheInitialized = true;
    
    // Pre-resolve common paths
    const pathsToResolve = ['/project', '/project/tsconfig.json', '/project/src'];
    for (const path of pathsToResolve) {
      try {
        const result = await resolver(path);
        if (result !== null) {
          resolverCache.set(path, result);
        }
      } catch (e) {
        // Ignore errors
      }
    }
  };
  
  // Initialize the cache before calling the native function
  await initializeCache();
  
  // Create a synchronous resolver that uses the cache
  const syncResolver = (path: string): FileResolverResult => {
    // Check cache first
    if (resolverCache.has(path)) {
      return resolverCache.get(path)!;
    }
    
    // Try the provided resolver (if it's sync)
    const result = resolver(path);
    if (result && typeof (result as any).then !== 'function') {
      // It's synchronous
      return result as FileResolverResult;
    }
    
    // For async results or cache misses, we need to handle specially
    // For tsconfig.json, provide a default
    if (path === '/project/tsconfig.json') {
      const configWithDefaults = { ...config };
      configWithDefaults.include = configWithDefaults.include || ['**/*'];
      const result = { type: 'file' as const, content: JSON.stringify(configWithDefaults, null, 2) };
      resolverCache.set(path, result);
      return result;
    }
    
    // For project root, ensure tsconfig.json is included
    if (path === '/project') {
      const result = { type: 'directory' as const, files: ['tsconfig.json'] };
      resolverCache.set(path, result);
      return result;
    }
    
    return null;
  };
  
  // Pre-cache more paths based on the initial results
  if (resolverCache.has('/project/src')) {
    const srcResult = resolverCache.get('/project/src');
    if (srcResult && srcResult.type === 'directory') {
      // Pre-cache source files
      for (const file of srcResult.files) {
        const filePath = `/project/src/${file}`;
        try {
          const fileResult = await resolver(filePath);
          if (fileResult !== null) {
            resolverCache.set(filePath, fileResult);
          }
        } catch (e) {
          // Ignore errors
        }
      }
    }
  }
  
  // Use the sync resolver with the native function
  // The native function expects: projectPath, printErrors, configFile, resolver, callback
  // After promisify, the callback is handled internally
  return buildWithDynamicResolverAsync(
    projectPath,
    false, // printErrors
    '', // configFile - empty like Swift does it
    syncResolver
  );
}


// Default TypeScript configuration
export const defaultConfig: TSConfig = {
  compilerOptions: {
    target: ECMAScriptTarget.ES2020,
    module: ModuleKind.CommonJS,
    outDir: './dist',
    rootDir: './src',
    strict: true,
    esModuleInterop: true,
    skipLibCheck: true,
    forceConsistentCasingInFileNames: true
  },
  exclude: ['node_modules', 'dist'],
  include: ['src/**/*']
};

// Node.js project configuration preset
export const nodeProjectConfig: TSConfig = {
  compilerOptions: {
    target: ECMAScriptTarget.ES2020,
    module: ModuleKind.CommonJS,
    outDir: './dist',
    rootDir: './src',
    strict: true,
    esModuleInterop: true,
    skipLibCheck: true,
    forceConsistentCasingInFileNames: true,
    declaration: true,
    sourceMap: true,
    resolveJsonModule: true
  },
  exclude: ['node_modules', 'dist', '**/*.test.ts', '**/*.spec.ts'],
  include: ['src/**/*']
};

// Re-export all types
export * from './types.js';