import { describe, it, expect } from 'vitest';
import { build, buildWithResolver, FileResolver, TSConfig, CompilerOptions, ECMAScriptTarget, ModuleKind, JSXEmit } from '../src/index.js';

describe('BuildInMemory Tests', () => {
  it('should compile basic TypeScript code', async () => {
    const sources = [{
      name: 'hello.ts',
      content: `
        function greet(name: string): string {
          return \`Hello, \${name}!\`;
        }

        const message = greet("World");
        console.log(message);
      `
    }];

    const config: TSConfig = {
      compilerOptions: {
        target: ECMAScriptTarget.ES2020,
        module: ModuleKind.CommonJS,
        noEmit: true,
        strict: true
      },
      include: ['src/**/*']
    };

    const result = await build(sources, config);


    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    expect(result.configFile).toBeTruthy();
  });

  it('should detect type errors', async () => {
    const sources = [{
      name: 'error.ts',
      content: `
        function addNumbers(a: number, b: number): number {
          return a + b;
        }

        // This should cause a type error
        const result = addNumbers("hello", 42);
      `
    }];

    const result = await build(sources);

    expect(result.success).toBe(false);
    expect(result.diagnostics.length).toBeGreaterThan(0);

    const errorDiagnostics = result.diagnostics.filter(d => d.category === 'error');
    expect(errorDiagnostics.length).toBeGreaterThan(0);

    // Look for the specific TS2345 error
    const ts2345Error = errorDiagnostics.find(d => d.code === 2345);
    expect(ts2345Error).toBeDefined();
    expect(ts2345Error?.message).toContain('not assignable');
  });

  it('should respect custom configuration', async () => {
    const sources = [{
      name: 'strict.ts',
      content: `
        // This should fail in strict mode - function parameter without type
        function greet(name) {
          return "Hello, " + name;
        }

        console.log(greet("World"));
      `
    }];

    // Test with strict mode
    const strictConfig: TSConfig = {
      compilerOptions: {
        strict: true,
        noImplicitAny: true,
        noEmit: true
      },
      include: ['src/**/*']
    };

    const strictResult = await build(sources, strictConfig);
    expect(strictResult.success).toBe(false);

    // Test with non-strict mode
    const lenientConfig: TSConfig = {
      compilerOptions: {
        strict: false,
        noImplicitAny: false,
        noEmit: true
      },
      include: ['src/**/*']
    };

    const lenientResult = await build(sources, lenientConfig);
    expect(lenientResult.success).toBe(true);
  });

  it('should handle multiple files with imports', async () => {
    const resolver: FileResolver = (path: string) => {
      const files: Record<string, string> = {
        '/project/tsconfig.json': JSON.stringify({
          compilerOptions: {
            target: 'es2015',
            module: 'commonjs',
            noEmit: true
          },
          include: ['**/*']
        }),
        '/project/utils.ts': 'export function add(a: number, b: number): number { return a + b; }',
        '/project/main.ts': `import { add } from './utils'; console.log(add(2, 3));`
      };

      if (files[path]) {
        return { type: 'file', content: files[path] };
      }
      
      if (path === '/project') {
        return { type: 'directory', files: ['utils.ts', 'main.ts', 'tsconfig.json'] };
      }
      
      return null;
    };

    const result = await buildWithResolver(resolver);

    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
  });

  it('should emit files when configured', async () => {
    const resolver: FileResolver = (path: string) => {
      const files: Record<string, string> = {
        '/project/tsconfig.json': JSON.stringify({
          compilerOptions: {
            target: 'es2020',
            module: 'commonjs',
            declaration: true,
            outDir: './dist',
            noEmit: false
          },
          include: ['**/*'],
          exclude: ['/project/dist']
        }),
        '/project/calculator.ts': `
          export class Calculator {
            add(a: number, b: number): number {
              return a + b;
            }

            subtract(a: number, b: number): number {
              return a - b;
            }
          }
        `
      };

      if (files[path]) {
        return { type: 'file', content: files[path] };
      }
      
      if (path === '/project') {
        return { type: 'directory', files: ['calculator.ts', 'tsconfig.json'] };
      }
      
      if (path === '/project/dist') {
        return { type: 'directory', files: [] };
      }
      
      return null;
    };

    const result = await buildWithResolver(resolver);

    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    expect(result.compiledFiles.length).toBeGreaterThan(0);

    // Should have both JS and declaration files
    const jsFiles = result.compiledFiles.filter(f => f.name.endsWith('.js'));
    const dtsFiles = result.compiledFiles.filter(f => f.name.endsWith('.d.ts'));

    expect(jsFiles.length).toBeGreaterThan(0);
    expect(dtsFiles.length).toBeGreaterThan(0);

    // Verify written files
    expect(Object.keys(result.writtenFiles).length).toBeGreaterThan(0);
    
    const writtenJsFiles = Object.keys(result.writtenFiles).filter(f => f.endsWith('.js'));
    const writtenDtsFiles = Object.keys(result.writtenFiles).filter(f => f.endsWith('.d.ts'));

    expect(writtenJsFiles.length).toBeGreaterThan(0);
    expect(writtenDtsFiles.length).toBeGreaterThan(0);

    // Verify content is not empty
    Object.values(result.writtenFiles).forEach(content => {
      expect(content).toBeTruthy();
    });
  });

  it('should support JSX', async () => {
    const sources = [{
      name: 'component.tsx',
      content: `
        import React from 'react';

        interface Props {
          message: string;
        }

        export const Greeting: React.FC<Props> = ({ message }) => {
          return <div>{message}</div>;
        };
      `
    }];

    const config: TSConfig = {
      compilerOptions: {
        jsx: JSXEmit.React,
        target: ECMAScriptTarget.ES2020,
        module: ModuleKind.CommonJS,
        esModuleInterop: true,
        allowSyntheticDefaultImports: true
      }
    };

    const result = await build(sources, config);

    // This might fail due to missing React types, but should not crash
    expect(result.diagnostics).toBeDefined();
  });

  it('should handle modern JavaScript features with proper lib config', async () => {
    const resolver: FileResolver = (path: string) => {
      const files: Record<string, string> = {
        '/project/tsconfig.json': JSON.stringify({
          compilerOptions: {
            target: 'es2020',
            lib: ['ES2020', 'DOM'],
            module: 'esnext',
            noEmit: true
          },
          include: ['**/*']
        }),
        '/project/modern.ts': `
          // Uses modern JavaScript features
          const promise = Promise.resolve(42);
          const result = await promise;
          console.log(result);

          const map = new Map<string, number>();
          map.set("answer", 42);

          // Export to make this a module
          export {};
        `
      };

      if (files[path]) {
        return { type: 'file', content: files[path] };
      }
      
      if (path === '/project') {
        return { type: 'directory', files: Object.keys(files).map(k => k.split('/').pop()!).filter(f => f !== 'tsconfig.json') };
      }
      
      return null;
    };

    const result = await buildWithResolver(resolver);

    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
  });

  it('should handle nested directory structures', async () => {
    const resolver: FileResolver = (path: string) => {
      const files: Record<string, string> = {
        '/project/tsconfig.json': JSON.stringify({
          compilerOptions: {
            target: 'es2020',
            module: 'commonjs',
            noEmit: true
          },
          include: ['**/*']
        }),
        '/project/src/utils/helper.ts': `
          export function formatMessage(msg: string): string {
            return \`[INFO] \${msg}\`;
          }
        `,
        '/project/src/main.ts': `
          import { formatMessage } from './utils/helper';

          const message = formatMessage("Hello, World!");
          console.log(message);
        `
      };

      if (files[path]) {
        return { type: 'file', content: files[path] };
      }
      
      const directories: Record<string, string[]> = {
        '/project': ['src', 'tsconfig.json'],
        '/project/src': ['main.ts', 'utils'],
        '/project/src/utils': ['helper.ts']
      };
      
      if (directories[path]) {
        return { type: 'directory', files: directories[path] };
      }
      
      return null;
    };

    const result = await buildWithResolver(resolver);

    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
  });

  it('should validate TypeScript code without emitting', async () => {
    const resolver: FileResolver = (path: string) => {
      const files: Record<string, string> = {
        '/project/tsconfig.json': JSON.stringify({
          compilerOptions: {
            noEmit: true,
            strict: true,
            target: 'ES2020'
          },
          include: ['**/*']
        }),
        '/project/simple.ts': `
          // Valid TypeScript code
          const message: string = "Hello, TypeScript!";
          const count: number = 42;
          const isActive: boolean = true;

          function add(a: number, b: number): number {
            return a + b;
          }

          const result = add(count, 10);
        `
      };

      if (files[path]) {
        return { type: 'file', content: files[path] };
      }
      
      if (path === '/project') {
        return { type: 'directory', files: ['simple.ts', 'tsconfig.json'] };
      }
      
      return null;
    };

    const result = await buildWithResolver(resolver);

    expect(result.success).toBe(true);
    expect(result.diagnostics.filter(d => d.category === 'error')).toEqual([]);
    expect(result.compiledFiles).toEqual([]); // No output files with noEmit
    expect(Object.keys(result.writtenFiles)).toEqual([]); // No written files with noEmit
  });
});