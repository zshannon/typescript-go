#!/usr/bin/env node

import { build as tscgoBuild } from '../dist/index.js';
import ts from 'typescript';
import { writeFileSync, mkdirSync, rmSync } from 'fs';
import { join } from 'path';

// Test cases of increasing complexity
const testCases = {
  simple: {
    sources: [
      { name: 'index.ts', content: 'const x: number = 42;' }
    ],
    config: {
      compilerOptions: {
        target: 'ES2020',
        strict: true,
        noEmit: true
      }
    }
  },

  smallProject: {
    sources: [
      { 
        name: 'index.ts', 
        content: `
          import { add, multiply } from './utils';
          
          const result = add(5, 3);
          const product = multiply(result, 2);
          
          console.log(product);
        `
      },
      { 
        name: 'utils.ts', 
        content: `
          export function add(a: number, b: number): number {
            return a + b;
          }
          
          export function multiply(a: number, b: number): number {
            return a * b;
          }
        `
      }
    ],
    config: {
      compilerOptions: {
        target: 'ES2020',
        module: 'CommonJS',
        strict: true,
        noEmit: true
      },
      include: ['**/*']
    }
  },

  mediumProject: {
    sources: [
      { 
        name: 'index.ts', 
        content: `
          import { UserService } from './service';
          import { User } from './types';
          
          const service = new UserService();
          const users: User[] = service.getAllUsers();
          
          users.forEach(user => {
            console.log(\`\${user.name} (\${user.email})\`);
          });
        `
      },
      { 
        name: 'types.ts', 
        content: `
          export interface User {
            id: number;
            name: string;
            email: string;
            createdAt: Date;
            isActive: boolean;
          }
          
          export interface UserCreateRequest {
            name: string;
            email: string;
          }
          
          export type UserUpdateRequest = Partial<Omit<User, 'id' | 'createdAt'>>;
        `
      },
      { 
        name: 'service.ts', 
        content: `
          import { User, UserCreateRequest, UserUpdateRequest } from './types';
          
          export class UserService {
            private users: User[] = [];
            
            getAllUsers(): User[] {
              return this.users.filter(user => user.isActive);
            }
            
            getUserById(id: number): User | undefined {
              return this.users.find(user => user.id === id);
            }
            
            createUser(request: UserCreateRequest): User {
              const user: User = {
                id: Date.now(),
                name: request.name,
                email: request.email,
                createdAt: new Date(),
                isActive: true
              };
              this.users.push(user);
              return user;
            }
            
            updateUser(id: number, updates: UserUpdateRequest): User | null {
              const userIndex = this.users.findIndex(user => user.id === id);
              if (userIndex === -1) return null;
              
              this.users[userIndex] = { ...this.users[userIndex], ...updates };
              return this.users[userIndex];
            }
          }
        `
      }
    ],
    config: {
      compilerOptions: {
        target: 'ES2020',
        module: 'CommonJS',
        strict: true,
        noEmit: true,
        lib: ['ES2020', 'DOM']
      },
      include: ['**/*']
    }
  }
};

// Benchmark tscgo (in-memory)
async function benchmarkTscgo(name, testCase, iterations = 10) {
  // Warm up
  await tscgoBuild(testCase.sources, testCase.config);
  
  const times = [];
  
  for (let i = 0; i < iterations; i++) {
    const start = process.hrtime.bigint();
    const result = await tscgoBuild(testCase.sources, testCase.config);
    const end = process.hrtime.bigint();
    
    const timeMs = Number(end - start) / 1_000_000;
    times.push(timeMs);
    
    if (!result.success) {
      console.error(`❌ ${name} failed:`, result.diagnostics.filter(d => d.category === 'error'));
      return null;
    }
  }
  
  const avg = times.reduce((a, b) => a + b, 0) / times.length;
  const min = Math.min(...times);
  const max = Math.max(...times);
  
  return { avg, min, max, times };
}

// Benchmark official TypeScript (file system)
function benchmarkTypeScript(name, testCase, iterations = 10) {
  const tempDir = `/tmp/tsbench-${Date.now()}`;
  
  try {
    // Create temp directory and files
    mkdirSync(tempDir, { recursive: true });
    
    // Write tsconfig.json
    const tsconfig = {
      compilerOptions: {
        target: ts.ScriptTarget.ES2020,
        module: ts.ModuleKind.CommonJS,
        strict: true,  
        noEmit: true,
        lib: testCase.config.compilerOptions.lib || ['ES2020']
      },
      include: ['**/*']
    };
    writeFileSync(join(tempDir, 'tsconfig.json'), JSON.stringify(tsconfig, null, 2));
    
    // Write source files
    for (const source of testCase.sources) {
      writeFileSync(join(tempDir, source.name), source.content);
    }
    
    // Parse tsconfig
    const configPath = join(tempDir, 'tsconfig.json');
    const configFile = ts.readConfigFile(configPath, ts.sys.readFile);
    const parsedConfig = ts.parseJsonConfigFileContent(
      configFile.config,
      ts.sys,
      tempDir
    );
    
    // Warm up
    const program = ts.createProgram(parsedConfig.fileNames, parsedConfig.options);
    const diagnostics = ts.getPreEmitDiagnostics(program);
    
    const times = [];
    
    for (let i = 0; i < iterations; i++) {
      const start = process.hrtime.bigint();
      
      const program = ts.createProgram(parsedConfig.fileNames, parsedConfig.options);
      const diagnostics = ts.getPreEmitDiagnostics(program);
      
      const end = process.hrtime.bigint();
      
      const timeMs = Number(end - start) / 1_000_000;
      times.push(timeMs);
      
      if (diagnostics.length > 0) {
        const errors = diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error);
        if (errors.length > 0) {
          console.error(`❌ ${name} failed:`, errors.map(d => d.messageText));
          return null;
        }
      }
    }
    
    const avg = times.reduce((a, b) => a + b, 0) / times.length;
    const min = Math.min(...times);
    const max = Math.max(...times);
    
    return { avg, min, max, times };
    
  } finally {
    // Clean up temp directory
    try {
      rmSync(tempDir, { recursive: true, force: true });
    } catch (e) {
      // Ignore cleanup errors
    }
  }
}

async function runComparison() {
  console.log('🚀 TypeScript Performance Comparison\n');
  console.log(`TypeScript version: ${ts.version}\n`);
  
  for (const [testName, testCase] of Object.entries(testCases)) {
    console.log(`📊 ${testName}:`);
    
    // Benchmark tscgo
    const tscgoResults = await benchmarkTscgo(testName, testCase);
    if (!tscgoResults) continue;
    
    // Benchmark official TypeScript
    const tsResults = benchmarkTypeScript(testName, testCase);
    if (!tsResults) continue;
    
    // Calculate speedup
    const speedup = tsResults.avg / tscgoResults.avg;
    
    console.log(`   📦 tscgo (in-memory):`);
    console.log(`      Average: ${tscgoResults.avg.toFixed(2)}ms`);
    console.log(`      Range: ${tscgoResults.min.toFixed(2)}-${tscgoResults.max.toFixed(2)}ms`);
    
    console.log(`   🔷 TypeScript (filesystem):`);
    console.log(`      Average: ${tsResults.avg.toFixed(2)}ms`);
    console.log(`      Range: ${tsResults.min.toFixed(2)}-${tsResults.max.toFixed(2)}ms`);
    
    console.log(`   ⚡ Speedup: ${speedup.toFixed(2)}x ${speedup > 1 ? 'faster' : 'slower'}`);
    console.log();
  }
  
  console.log('🏁 Comparison complete!');
}

runComparison().catch(console.error);