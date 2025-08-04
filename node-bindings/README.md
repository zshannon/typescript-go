# @zshannon/tscgo

High-performance TypeScript compiler bindings for Node.js using Microsoft's Go implementation of the TypeScript compiler.

## ⚡ Performance

**5-9x faster** than the official TypeScript compiler:

- **Simple projects**: ~100ms vs ~500ms (5x faster)
- **Medium projects**: ~100ms vs ~900ms (9x faster)
- **Consistent low latency**: Sub-100ms for most use cases

## 🚀 Features

- **In-memory compilation**: No filesystem I/O overhead
- **Full TypeScript support**: All language features and strict mode
- **ESM modules**: Modern JavaScript module system
- **Type-safe API**: Complete TypeScript definitions
- **Cross-platform**: macOS, Linux (glibc-based distributions)

## 📦 Installation

```bash
npm install @zshannon/tscgo
```

## 🔧 Usage

### Basic Type Checking

```javascript
import { build } from '@zshannon/tscgo';

const sources = [
  { name: 'index.ts', content: 'const x: number = 42;' }
];

const config = {
  compilerOptions: {
    target: 'ES2020',
    strict: true,
    noEmit: true
  }
};

const result = await build(sources, config);

if (result.success) {
  console.log('✅ Type checking passed');
} else {
  console.log('❌ Type errors:', result.diagnostics);
}
```

### Multi-file Projects

```javascript
const sources = [
  { 
    name: 'index.ts', 
    content: `
      import { add } from './utils';
      const result = add(5, 3);
    `
  },
  { 
    name: 'utils.ts', 
    content: `
      export function add(a: number, b: number): number {
        return a + b;
      }
    `
  }
];

const config = {
  compilerOptions: {
    target: 'ES2020',
    module: 'CommonJS',
    strict: true
  },
  include: ['**/*']
};

const result = await build(sources, config);
```

### Dynamic File Resolution

```javascript
import { buildWithResolver } from '@zshannon/tscgo';

const resolver = async (path) => {
  // Load files from database, HTTP, etc.
  const content = await loadFileFromDatabase(path);
  
  if (!content) return null;
  
  return {
    content,
    isFile: true,
    isDirectory: false
  };
};

const result = await buildWithResolver(resolver);
```

## 🛠️ Development

### Building from Source

```bash
# Build Go library and native addon
npm run build

# Run tests
npm test

# Clean build artifacts
npm run clean
```

### Requirements

- **Node.js**: 18.0.0 or higher
- **Go**: 1.21 or higher (for building from source)
- **Python**: For node-gyp native compilation
- **C++ compiler**: GCC, Clang, or MSVC

## 🌍 Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| macOS (Intel/ARM) | ✅ Supported | Full functionality |
| Linux (glibc) | ✅ Supported | Ubuntu, Debian, RHEL, etc. |
| Linux (musl) | ❌ Not supported | Alpine Linux incompatible (TLS issues) |
| Windows | 🚧 Untested | Should work but needs testing |

## 📊 Benchmarks

Run the included benchmark to compare performance:

```bash
node examples/benchmark-comparison.js
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## 📄 License

MIT

## 🔗 Related

- [Microsoft TypeScript-Go](https://github.com/microsoft/typescript-go) - The underlying Go implementation
- [TypeScript](https://github.com/microsoft/TypeScript) - Official TypeScript compiler

---

Built with ❤️ using Microsoft's TypeScript-Go compiler implementation.