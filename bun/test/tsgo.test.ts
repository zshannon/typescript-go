import {
    describe,
    expect,
    test,
} from "bun:test";
import tsgo from "../src/index";

// Virtual file paths for type checking - these don't hit the filesystem
const ts = (name: string) => `/virtual/${name}.ts`;
const tsx = (name: string) => `/virtual/${name}.tsx`;

describe("tsgo", () => {
    describe("version", () => {
        test("returns version string", () => {
            const version = tsgo.version();
            expect(version).toBe("1.0.0");
        });
    });

    describe("typecheck - basic TypeScript", () => {
        test("valid code passes", () => {
            const code = `
        const x: number = 42;
        const y: string = "hello";
        const z: boolean = true;
      `;
            const result = tsgo.typecheck(code, ts("basic"));
            expect(result.success).toBe(true);
            expect(result.diagnostics?.length ?? 0).toBe(0);
        });

        test("type mismatch fails", () => {
            const code = `
        const x: number = "not a number";
      `;
            const result = tsgo.typecheck(code, ts("error"));
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
            const result = tsgo.typecheck(code, ts("interface"));
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
            const result = tsgo.typecheck(code, ts("missing-prop"));
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
            const result = tsgo.typecheck(code, ts("generics"));
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
            const result = tsgo.typecheck(code, ts("class"));
            expect(result.success).toBe(true);
        });
    });

    describe("typecheck - React", () => {
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
            const result = tsgo.typecheck(code, tsx("Greeting"));
            expect(result.success).toBe(true);
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
            const result = tsgo.typecheck(code, tsx("MissingProp"));
            expect(result.success).toBe(false);
            // Error should mention the type mismatch (ButtonProps or the missing property)
            expect(result.diagnostics?.some(d => d.message.includes("ButtonProps") || d.message.includes("onClick") || d.message.includes("label"))).toBe(true);
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
            const result = tsgo.typecheck(code, tsx("Hooks"));
            expect(result.success).toBe(true);
        });
    });

    describe("typecheckWithOptions - custom JSX", () => {
        test("jsxImportSource option is respected", () => {
            // Should fail because @emotion/react isn't installed
            const code = `
        const App = () => <div>Hello</div>;
        export default App;
      `;
            const result = tsgo.typecheckWithOptions(code, tsx("Emotion"), {
                jsx: "react-jsx",
                jsxImportSource: "@emotion/react",
                strict: true,
                skipLibCheck: true,
                target: "ES2022",
                module: "ESNext",
                moduleResolution: "Bundler",
                lib: ["ES2022", "DOM"],
            });
            expect(result.success).toBe(false);
            expect(result.diagnostics?.some(d => d.message.includes("@emotion/react/jsx-runtime"))).toBe(true);
        });

        test("custom JSX runtime with unique elements succeeds", () => {
            // Using unique elements that don't exist in react-dom
            const code = `
        const App = () => {
          return (
            <zframe layout="centered">
              <zheading level={1}>Hello Mini JSX!</zheading>
              <zstack direction="horizontal" gap={8}>
                <ztext color="blue" bold>Custom JSX runtime works!</ztext>
                <zbutton variant="primary" onTap={() => console.log("tapped")}>
                  Tap me
                </zbutton>
              </zstack>
            </zframe>
          );
        };

        export default App;
      `;
            const result = tsgo.typecheckWithOptions(code, tsx("MiniApp"), {
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
        });

        test("custom JSX runtime fails with unknown element", () => {
            // Using 'div' which doesn't exist in @mini/jsx-runtime
            const code = `
        const App = () => {
          return (
            <div>This should fail - div is not in @mini/jsx-runtime!</div>
          );
        };

        export default App;
      `;
            const result = tsgo.typecheckWithOptions(code, tsx("BadElement"), {
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
            expect(result.diagnostics?.some(d => d.message.includes("div"))).toBe(true);
        });

        test("custom JSX runtime fails with wrong prop type", () => {
            // level should be number, not string
            const code = `
        const App = () => {
          return (
            <zheading level="wrong">Wrong type!</zheading>
          );
        };

        export default App;
      `;
            const result = tsgo.typecheckWithOptions(code, tsx("BadProp"), {
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
            expect(result.diagnostics?.length).toBeGreaterThan(0);
        });
    });

    describe("typecheckMultiple - multi-file projects", () => {
        test("cross-file imports work", () => {
            const files = {
                [ts("types")]: `
          export interface User {
            id: number;
            name: string;
          }
        `,
                [ts("utils")]: `
          import { User } from './types';

          export function greet(user: User): string {
            return \`Hello, \${user.name}!\`;
          }
        `,
                [ts("main")]: `
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
                [ts("types")]: `
          export interface User {
            id: number;
            name: string;
          }
        `,
                [ts("main")]: `
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
                [ts("types")]: `
          export interface Todo {
            id: number;
            text: string;
            completed: boolean;
          }
        `,
                [tsx("TodoItem")]: `
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
                [tsx("App")]: `
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

    describe("diagnostics", () => {
        test("reports correct line number", () => {
            const code = `const x: number = 1;
const y: number = "wrong";
const z: number = 3;`;
            const result = tsgo.typecheck(code, ts("lines"));
            expect(result.success).toBe(false);
            expect(result.diagnostics?.[0]?.line).toBe(2);
        });

        test("reports error category", () => {
            const code = `const x: number = "wrong";`;
            const result = tsgo.typecheck(code, ts("category"));
            expect(result.diagnostics?.[0]?.category).toBe("error");
        });

        test("reports error code", () => {
            const code = `const x: number = "wrong";`;
            const result = tsgo.typecheck(code, ts("code"));
            expect(result.diagnostics?.[0]?.code).toBe(2322);
        });
    });

    describe("performance", () => {
        test("reports duration", () => {
            const code = `const x = 1;`;
            const result = tsgo.typecheck(code, ts("perf"));
            expect(result.duration_ms).toBeGreaterThan(0);
            expect(result.duration_ms).toBeLessThan(5000);
        });
    });
});
