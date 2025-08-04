import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { buildFromFileSystem, BuildOptions } from '../src/index.js';
import { promises as fs } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import os from 'os';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

describe('BuildFileSystem Tests', () => {
  let tempDir: string;
  
  beforeAll(async () => {
    // Create a temporary directory for test projects
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'tscgo-test-'));
  });
  
  afterAll(async () => {
    // Clean up temporary directory
    await fs.rm(tempDir, { recursive: true, force: true });
  });
  
  async function createTestProject(name: string, files: Record<string, string>): Promise<string> {
    const projectDir = path.join(tempDir, name);
    await fs.mkdir(projectDir, { recursive: true });
    
    for (const [filePath, content] of Object.entries(files)) {
      const fullPath = path.join(projectDir, filePath);
      await fs.mkdir(path.dirname(fullPath), { recursive: true });
      await fs.writeFile(fullPath, content, 'utf8');
    }
    
    return projectDir;
  }
  
  it('should compile a hello world project', async () => {
    const projectPath = await createTestProject('test-hello', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          outDir: './dist',
          rootDir: './src',
          strict: true,
          esModuleInterop: true,
          skipLibCheck: true,
          forceConsistentCasingInFileNames: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/hello.ts': `
function greet(name: string): string {
  return \`Hello, \${name}!\`;
}

const message = greet("World");
console.log(message);

export { greet };
      `
    });
    
    const result = buildFromFileSystem(projectPath, { printErrors: false });
    
    expect(result.success).toBe(true);
    expect(result.diagnostics).toEqual([]);
    expect(result.configFile).toBeTruthy();
    expect(result.configFile).toContain('tsconfig.json');
    
    // Verify output file exists
    const outputFile = path.join(projectPath, 'dist', 'hello.js');
    const outputExists = await fs.access(outputFile).then(() => true).catch(() => false);
    expect(outputExists).toBe(true);
    
    if (outputExists) {
      const outputContent = await fs.readFile(outputFile, 'utf8');
      expect(outputContent).toContain('console.log(message)');
    }
  });
  
  it('should detect type errors', async () => {
    const projectPath = await createTestProject('test-error', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          outDir: './dist',
          rootDir: './src',
          strict: true,
          noEmitOnError: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/error.ts': `
function add(a: number, b: number): number {
  return a + b;
}

// This should cause a type error - passing string to number parameter
const result = add("hello", 5);

console.log(result);

export { add };
      `
    });
    
    const result = buildFromFileSystem(projectPath, { printErrors: false });
    
    expect(result.success).toBe(false);
    expect(result.configFile).toBeTruthy();
    expect(result.diagnostics.length).toBeGreaterThan(0);
    
    // Check that we have the expected TypeScript error
    const errorDiagnostics = result.diagnostics.filter(d => d.category === 'error');
    expect(errorDiagnostics.length).toBeGreaterThan(0);
    
    // Look for the specific TS2345 error
    const ts2345Error = errorDiagnostics.find(d => d.code === 2345);
    expect(ts2345Error).toBeDefined();
    expect(ts2345Error?.message).toContain('Argument of type \'string\' is not assignable to parameter of type \'number\'');
    
    // Verify the error has proper location information
    if (ts2345Error) {
      expect(ts2345Error.file).toBeTruthy();
      expect(ts2345Error.file).toContain('error.ts');
      expect(ts2345Error.line).toBeGreaterThan(0);
      expect(ts2345Error.column).toBeGreaterThan(0);
    }
  });
  
  it('should provide detailed diagnostics', async () => {
    const projectPath = await createTestProject('test-detailed-error', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          strict: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/errors.ts': `
// Multiple errors
function badFunction(x: number): string {
  return x; // Error: number not assignable to string
}

const y: boolean = "not a boolean"; // Error: string not assignable to boolean
      `
    });
    
    const result = buildFromFileSystem(projectPath, { printErrors: true });
    
    expect(result.success).toBe(false);
    expect(result.diagnostics.length).toBeGreaterThan(0);
    
    // Check diagnostic details
    result.diagnostics.forEach(diagnostic => {
      expect(diagnostic.message).toBeTruthy();
      expect(diagnostic.category).toBeTruthy();
      
      if (diagnostic.category === 'error') {
        expect(diagnostic.code).toBeGreaterThan(0);
        expect(diagnostic.file).toBeTruthy();
        expect(diagnostic.line).toBeGreaterThanOrEqual(0);
        expect(diagnostic.column).toBeGreaterThanOrEqual(0);
      }
    });
  });
  
  it('should emit files when configured', async () => {
    const projectPath = await createTestProject('test-emit', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          outDir: './dist',
          rootDir: './src',
          declaration: true,
          sourceMap: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/math.ts': `
export function add(a: number, b: number): number {
  return a + b;
}

export function multiply(a: number, b: number): number {
  return a * b;
}
      `
    });
    
    const result = buildFromFileSystem(projectPath);
    
    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    
    // Check that output files were created
    const distDir = path.join(projectPath, 'dist');
    const distExists = await fs.access(distDir).then(() => true).catch(() => false);
    expect(distExists).toBe(true);
    
    if (distExists) {
      const distFiles = await fs.readdir(distDir);
      expect(distFiles.length).toBeGreaterThan(0);
      
      // Should have JS, declaration, and source map files
      expect(distFiles.some(f => f.endsWith('.js'))).toBe(true);
      expect(distFiles.some(f => f.endsWith('.d.ts'))).toBe(true);
      expect(distFiles.some(f => f.endsWith('.js.map'))).toBe(true);
    }
  });
  
  it('should use explicit config file', async () => {
    const projectPath = await createTestProject('test-custom-config', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2015',
          module: 'commonjs'
        }
      }, null, 2),
      'custom.config.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          outDir: './build',
          strict: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/index.ts': `
const message: string = "Using custom config";
console.log(message);
      `
    });
    
    const result = buildFromFileSystem(projectPath, {
      printErrors: false,
      configFile: path.join(projectPath, 'custom.config.json')
    });
    
    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    expect(result.configFile).toBeTruthy();
    expect(result.configFile).toContain('custom.config.json');
    
    // Check that output went to 'build' directory, not 'dist'
    const buildDir = path.join(projectPath, 'build');
    const buildExists = await fs.access(buildDir).then(() => true).catch(() => false);
    expect(buildExists).toBe(true);
  });
  
  it('should handle non-existent project path', async () => {
    const nonExistentPath = path.join(tempDir, 'does-not-exist');
    
    const result = buildFromFileSystem(nonExistentPath, { printErrors: false });
    
    expect(result.success).toBe(false);
    expect(result.diagnostics.length).toBeGreaterThan(0);
    
    const errorDiagnostics = result.diagnostics.filter(d => d.category === 'error');
    expect(errorDiagnostics.length).toBeGreaterThan(0);
  });
  
  it('should handle validation-only builds', async () => {
    const projectPath = await createTestProject('test-validation', {
      'tsconfig.json': JSON.stringify({
        compilerOptions: {
          target: 'es2020',
          module: 'commonjs',
          noEmit: true,
          strict: true
        },
        include: ['src/**/*']
      }, null, 2),
      'src/validate.ts': `
interface User {
  id: number;
  name: string;
  email?: string;
}

function validateUser(user: unknown): user is User {
  return typeof user === 'object' &&
         user !== null &&
         'id' in user &&
         'name' in user;
}

const user: unknown = { id: 1, name: "John" };
if (validateUser(user)) {
  console.log(user.name); // TypeScript knows this is safe
}
      `
    });
    
    const result = buildFromFileSystem(projectPath);
    
    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    
    // Should not create any output files with noEmit
    const distDir = path.join(projectPath, 'dist');
    const distExists = await fs.access(distDir).then(() => true).catch(() => false);
    expect(distExists).toBe(false);
  });
});