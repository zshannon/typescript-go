import { expect, test, describe } from "bun:test";
import tsgo from "./tsgo";

describe("tsgo FFI Type Checker", () => {
  describe("version", () => {
    test("returns version string", () => {
      const version = tsgo.version();
      expect(version).toBe("1.0.0");
    });
  });

  describe("basic TypeScript", () => {
    test("valid code passes", () => {
      const code = `
        const x: number = 42;
        const y: string = "hello";
        const z: boolean = true;
      `;
      const result = tsgo.typecheck(code, "/project/basic.ts");
      expect(result.success).toBe(true);
      expect(result.diagnostics?.length ?? 0).toBe(0);
    });

    test("type mismatch fails", () => {
      const code = `
        const x: number = "not a number";
      `;
      const result = tsgo.typecheck(code, "/project/error.ts");
      expect(result.success).toBe(false);
      expect(result.diagnostics?.length).toBeGreaterThan(0);
      expect(result.diagnostics?.[0]?.message).toContain("Type 'string' is not assignable to type 'number'");
    });

    test("interface type checking", () => {
      const code = `
        interface User {
          id: number;
          name: string;
          email: string;
        }

        const user: User = {
          id: 1,
          name: "Alice",
          email: "alice@example.com"
        };
      `;
      const result = tsgo.typecheck(code, "/project/interface.ts");
      expect(result.success).toBe(true);
    });

    test("missing interface property fails", () => {
      const code = `
        interface User {
          id: number;
          name: string;
          email: string;
        }

        const user: User = {
          id: 1,
          name: "Alice"
          // missing email
        };
      `;
      const result = tsgo.typecheck(code, "/project/missing-prop.ts");
      expect(result.success).toBe(false);
      expect(result.diagnostics?.[0]?.message).toContain("email");
    });

    test("generic functions", () => {
      const code = `
        function identity<T>(value: T): T {
          return value;
        }

        const num = identity(42);
        const str = identity("hello");
        const numCheck: number = num;
        const strCheck: string = str;
      `;
      const result = tsgo.typecheck(code, "/project/generics.ts");
      expect(result.success).toBe(true);
    });

    test("class with private members", () => {
      const code = `
        class Counter {
          private count: number = 0;

          increment(): void {
            this.count++;
          }

          getCount(): number {
            return this.count;
          }
        }

        const counter = new Counter();
        counter.increment();
        const count: number = counter.getCount();
      `;
      const result = tsgo.typecheck(code, "/project/class.ts");
      expect(result.success).toBe(true);
    });
  });

  describe("React type checking", () => {
    test("valid React component passes", () => {
      const code = `
        import React from 'react';

        interface Props {
          name: string;
          age?: number;
        }

        const Greeting: React.FC<Props> = ({ name, age }) => {
          return (
            <div>
              <h1>Hello, {name}!</h1>
              {age && <p>Age: {age}</p>}
            </div>
          );
        };

        export default Greeting;
      `;
      const result = tsgo.typecheck(code, "/project/Greeting.tsx");
      expect(result.success).toBe(true);
      expect(result.diagnostics?.length ?? 0).toBe(0);
    });

    test("React component with missing required prop fails", () => {
      const code = `
        import React from 'react';

        interface ButtonProps {
          label: string;
          onClick: () => void;
        }

        const Button: React.FC<ButtonProps> = ({ label, onClick }) => {
          return <button onClick={onClick}>{label}</button>;
        };

        // Missing onClick prop
        const App = () => <Button label="Click me" />;
      `;
      const result = tsgo.typecheck(code, "/project/MissingProp.tsx");
      expect(result.success).toBe(false);
      expect(result.diagnostics?.some(d => d.message.includes("onClick"))).toBe(true);
    });

    test("React hooks type inference", () => {
      const code = `
        import React, { useState, useEffect, useCallback } from 'react';

        const Counter: React.FC = () => {
          const [count, setCount] = useState(0);
          const [name, setName] = useState<string>("Counter");

          useEffect(() => {
            document.title = \`\${name}: \${count}\`;
          }, [count, name]);

          const increment = useCallback(() => {
            setCount(prev => prev + 1);
          }, []);

          return (
            <div>
              <h1>{name}: {count}</h1>
              <button onClick={increment}>+</button>
            </div>
          );
        };

        export default Counter;
      `;
      const result = tsgo.typecheck(code, "/project/Hooks.tsx");
      expect(result.success).toBe(true);
    });

    test("JSX intrinsic elements", () => {
      const code = `
        import React from 'react';

        const Layout: React.FC = () => {
          return (
            <div className="container">
              <header>
                <nav>
                  <a href="/">Home</a>
                </nav>
              </header>
              <main>
                <h1>Welcome</h1>
                <p>This is a paragraph.</p>
                <input type="text" placeholder="Enter text" />
                <button type="submit">Submit</button>
              </main>
              <footer>
                <small>© 2024</small>
              </footer>
            </div>
          );
        };

        export default Layout;
      `;
      const result = tsgo.typecheck(code, "/project/Layout.tsx");
      expect(result.success).toBe(true);
    });
  });

  describe("custom JSX options", () => {
    test("jsxImportSource option is respected", () => {
      // This should fail because @emotion/react isn't installed
      // but it should try to look for @emotion/react/jsx-runtime
      const code = `
        const App = () => <div>Hello</div>;
        export default App;
      `;
      const result = tsgo.typecheckWithOptions(code, "/project/Emotion.tsx", {
        jsx: "react-jsx",
        jsxImportSource: "@emotion/react",
        strict: true,
        skipLibCheck: true,
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "Bundler",
        lib: ["ES2022", "DOM"],
      });
      // Should fail with specific error about @emotion/react
      expect(result.success).toBe(false);
      expect(result.diagnostics?.some(d =>
        d.message.includes("@emotion/react/jsx-runtime")
      )).toBe(true);
    });

    test("custom jsxImportSource with @mini/jsx-runtime succeeds", () => {
      // Test with our mini custom JSX runtime using UNIQUE element names
      // These elements (zframe, zheading, zbutton, ztext, zstack) DON'T exist in react-dom!
      // If this passes, it proves jsxImportSource is actually being used
      const code = `
        const App = () => {
          return (
            <zframe layout="centered">
              <zheading level={1}>Hello Mini JSX!</zheading>
              <zstack direction="horizontal" gap={8}>
                <ztext color="blue" bold>Welcome to custom JSX</ztext>
                <zbutton variant="primary" onTap={() => console.log("tapped")}>
                  Tap me
                </zbutton>
              </zstack>
            </zframe>
          );
        };

        export default App;
      `;
      const result = tsgo.typecheckWithOptions(code, "/project/MiniApp.tsx", {
        jsx: "react-jsx",
        jsxImportSource: "@mini/jsx-runtime",
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "Bundler",
        strict: true,
        skipLibCheck: true,
        lib: ["ES2022", "DOM"],
      });
      expect(result.success).toBe(true);
      expect(result.diagnostics?.length ?? 0).toBe(0);
    });

    test("custom jsxImportSource fails with unknown element", () => {
      // Using 'div' which does NOT exist in @mini/jsx-runtime (only zframe, ztext, etc.)
      // This MUST fail - proving that our custom JSX definitions are enforced
      const code = `
        const App = () => {
          return (
            <div>This should fail - div is not in @mini/jsx-runtime!</div>
          );
        };

        export default App;
      `;
      const result = tsgo.typecheckWithOptions(code, "/project/BadElement.tsx", {
        jsx: "react-jsx",
        jsxImportSource: "@mini/jsx-runtime",
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "Bundler",
        strict: true,
        skipLibCheck: true,
        lib: ["ES2022", "DOM"],
      });
      expect(result.success).toBe(false);
      // Should error because 'div' is not in IntrinsicElements
      expect(result.diagnostics?.some(d => d.message.includes("div"))).toBe(true);
    });

    test("custom jsxImportSource fails with wrong prop type", () => {
      // Using wrong prop type: level should be number, not string
      const code = `
        const App = () => {
          return (
            <zheading level="wrong">Wrong type!</zheading>
          );
        };

        export default App;
      `;
      const result = tsgo.typecheckWithOptions(code, "/project/BadProp.tsx", {
        jsx: "react-jsx",
        jsxImportSource: "@mini/jsx-runtime",
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "Bundler",
        strict: true,
        skipLibCheck: true,
        lib: ["ES2022", "DOM"],
      });
      expect(result.success).toBe(false);
      // Should error because level expects 1|2|3|4|5|6, not string
      expect(result.diagnostics?.length).toBeGreaterThan(0);
    });

    test("full server-compatible config works", () => {
      const code = `
        import React, { PropsWithChildren } from 'react';

        interface Props {
          title: string;
        }

        const Card: React.FC<PropsWithChildren<Props>> = ({ title, children }) => {
          return (
            <div className="card">
              <h2>{title}</h2>
              <div className="content">{children}</div>
            </div>
          );
        };

        export default Card;
      `;
      const result = tsgo.typecheckWithOptions(code, "/project/Card.tsx", {
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "Bundler",
        jsx: "react-jsx",
        strict: true,
        noEmit: true,
        skipLibCheck: true,
        allowJs: true,
        declaration: true,
        esModuleInterop: true,
        isolatedModules: true,
        resolveJsonModule: true,
        forceConsistentCasingInFileNames: true,
        lib: ["ES2022", "DOM"],
      });
      expect(result.success).toBe(true);
    });
  });

  describe("multi-file projects", () => {
    test("cross-file imports work", () => {
      const files = {
        "/project/types.ts": `
          export interface User {
            id: number;
            name: string;
          }
        `,
        "/project/utils.ts": `
          import { User } from './types';

          export function greet(user: User): string {
            return \`Hello, \${user.name}!\`;
          }
        `,
        "/project/main.ts": `
          import { User } from './types';
          import { greet } from './utils';

          const user: User = { id: 1, name: "Alice" };
          const greeting: string = greet(user);
        `,
      };
      const result = tsgo.typecheckMultiple(files, {
        strict: true,
        target: "ES2022",
      });
      expect(result.success).toBe(true);
    });

    test("cross-file type errors are caught", () => {
      const files = {
        "/project/types.ts": `
          export interface User {
            id: number;
            name: string;
          }
        `,
        "/project/main.ts": `
          import { User } from './types';

          // Missing name property
          const user: User = { id: 1 };
        `,
      };
      const result = tsgo.typecheckMultiple(files, {
        strict: true,
        target: "ES2022",
      });
      expect(result.success).toBe(false);
      expect(result.diagnostics?.some(d => d.message.includes("name"))).toBe(true);
    });

    test("React multi-file project", () => {
      const files = {
        "/project/types.ts": `
          export interface Todo {
            id: number;
            text: string;
            completed: boolean;
          }
        `,
        "/project/TodoItem.tsx": `
          import React from 'react';
          import { Todo } from './types';

          interface Props {
            todo: Todo;
            onToggle: (id: number) => void;
          }

          export const TodoItem: React.FC<Props> = ({ todo, onToggle }) => {
            return (
              <li onClick={() => onToggle(todo.id)}>
                <input type="checkbox" checked={todo.completed} readOnly />
                <span>{todo.text}</span>
              </li>
            );
          };
        `,
        "/project/App.tsx": `
          import React, { useState } from 'react';
          import { Todo } from './types';
          import { TodoItem } from './TodoItem';

          const App: React.FC = () => {
            const [todos, setTodos] = useState<Todo[]>([
              { id: 1, text: "Learn TypeScript", completed: false },
              { id: 2, text: "Build app", completed: false },
            ]);

            const toggleTodo = (id: number) => {
              setTodos(todos.map(t =>
                t.id === id ? { ...t, completed: !t.completed } : t
              ));
            };

            return (
              <ul>
                {todos.map(todo => (
                  <TodoItem key={todo.id} todo={todo} onToggle={toggleTodo} />
                ))}
              </ul>
            );
          };

          export default App;
        `,
      };
      const result = tsgo.typecheckMultiple(files, {
        jsx: "react-jsx",
        strict: true,
        target: "ES2022",
      });
      expect(result.success).toBe(true);
    });
  });

  describe("error diagnostics", () => {
    test("reports correct line and column", () => {
      const code = `const x: number = 1;
const y: number = "wrong";
const z: number = 3;`;
      const result = tsgo.typecheck(code, "/project/lines.ts");
      expect(result.success).toBe(false);
      expect(result.diagnostics?.length).toBeGreaterThan(0);

      const diag = result.diagnostics?.[0];
      expect(diag?.line).toBe(2); // Error is on line 2
    });

    test("reports error category", () => {
      const code = `const x: number = "wrong";`;
      const result = tsgo.typecheck(code, "/project/category.ts");
      expect(result.diagnostics?.[0]?.category).toBe("error");
    });

    test("reports error code", () => {
      const code = `const x: number = "wrong";`;
      const result = tsgo.typecheck(code, "/project/code.ts");
      expect(result.diagnostics?.[0]?.code).toBe(2322); // TS2322: Type 'string' is not assignable to type 'number'
    });
  });

  describe("performance", () => {
    test("reports duration", () => {
      const code = `const x = 1;`;
      const result = tsgo.typecheck(code, "/project/perf.ts");
      expect(result.duration_ms).toBeGreaterThan(0);
      expect(result.duration_ms).toBeLessThan(5000); // Should complete in under 5s
    });
  });
});
