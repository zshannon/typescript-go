// Minimal React type definitions for testing
declare namespace React {
  type ReactNode = React.ReactElement | JSX.Element | string | number | boolean | null | undefined | ReactNode[];

  interface ReactElement<P = any> {
    type: string | ComponentType<P>;
    props: P;
    key: string | null;
  }

  type ComponentType<P = {}> = FunctionComponent<P> | ComponentClass<P>;

  interface FunctionComponent<P = {}> {
    (props: P): ReactElement<any, any> | JSX.Element | null;
    displayName?: string;
  }

  type FC<P = {}> = FunctionComponent<P>;

  interface ComponentClass<P = {}> {
    new(props: P): Component<P>;
  }

  class Component<P = {}, S = {}> {
    props: Readonly<P>;
    state: Readonly<S>;
    setState(state: Partial<S>): void;
    render(): ReactNode;
  }

  type PropsWithChildren<P = unknown> = P & { children?: ReactNode };

  // Hooks
  function useState<T>(initialState: T | (() => T)): [T, (value: T | ((prev: T) => T)) => void];
  function useEffect(effect: () => void | (() => void), deps?: readonly unknown[]): void;
  function useCallback<T extends (...args: any[]) => any>(callback: T, deps: readonly unknown[]): T;
  function useMemo<T>(factory: () => T, deps: readonly unknown[]): T;
  function useRef<T>(initialValue: T): { current: T };
  function useContext<T>(context: Context<T>): T;

  interface Context<T> {
    Provider: ComponentType<{ value: T; children?: ReactNode }>;
    Consumer: ComponentType<{ children: (value: T) => ReactNode }>;
  }

  function createContext<T>(defaultValue: T): Context<T>;
}

declare namespace JSX {
  interface Element extends React.ReactElement<any, any> {}

  interface IntrinsicElements {
    div: React.HTMLAttributes<HTMLDivElement>;
    span: React.HTMLAttributes<HTMLSpanElement>;
    p: React.HTMLAttributes<HTMLParagraphElement>;
    h1: React.HTMLAttributes<HTMLHeadingElement>;
    h2: React.HTMLAttributes<HTMLHeadingElement>;
    h3: React.HTMLAttributes<HTMLHeadingElement>;
    button: React.ButtonHTMLAttributes<HTMLButtonElement>;
    input: React.InputHTMLAttributes<HTMLInputElement>;
    li: React.LiHTMLAttributes<HTMLLIElement>;
    ul: React.HTMLAttributes<HTMLUListElement>;
    a: React.AnchorHTMLAttributes<HTMLAnchorElement>;
  }
}

declare namespace React {
  interface HTMLAttributes<T> {
    className?: string;
    id?: string;
    style?: Record<string, string | number>;
    onClick?: (event: any) => void;
    children?: ReactNode;
    key?: string | number;
  }

  interface ButtonHTMLAttributes<T> extends HTMLAttributes<T> {
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
  }

  interface InputHTMLAttributes<T> extends HTMLAttributes<T> {
    type?: string;
    value?: string | number;
    checked?: boolean;
    readOnly?: boolean;
    onChange?: (event: any) => void;
  }

  interface LiHTMLAttributes<T> extends HTMLAttributes<T> {
    value?: string | number;
  }

  interface AnchorHTMLAttributes<T> extends HTMLAttributes<T> {
    href?: string;
    target?: string;
  }
}

export = React;
export as namespace React;
