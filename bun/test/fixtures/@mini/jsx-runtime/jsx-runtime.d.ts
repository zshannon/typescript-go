/**
 * Mini JSX Runtime - A minimal custom JSX runtime for testing
 * Uses UNIQUE element names that don't exist in react-dom to prove jsxImportSource works
 */

export namespace JSX {
  interface IntrinsicElements {
    // Unique elements - these DON'T exist in react-dom!
    zframe: {
      layout?: string;
      id?: string;
      style?: Record<string, string | number>;
      onTap?: () => void;
      children?: Element | Element[] | string | number;
    };
    ztext: {
      color?: string;
      size?: "small" | "medium" | "large";
      bold?: boolean;
      children?: Element | Element[] | string | number;
    };
    zbutton: {
      variant?: "primary" | "secondary" | "danger";
      disabled?: boolean;
      onTap?: () => void;
      children?: Element | Element[] | string | number;
    };
    zheading: {
      level?: 1 | 2 | 3 | 4 | 5 | 6;
      children?: Element | Element[] | string | number;
    };
    zstack: {
      direction?: "horizontal" | "vertical";
      gap?: number;
      children?: Element | Element[] | string | number;
    };
  }

  interface Element {
    type: string | Function;
    props: Record<string, unknown>;
    key: string | null;
  }

  interface ElementChildrenAttribute {
    children: {};
  }
}

export function jsx(
  type: string | Function,
  props: Record<string, unknown>,
  key?: string
): JSX.Element;

export function jsxs(
  type: string | Function,
  props: Record<string, unknown>,
  key?: string
): JSX.Element;

export function jsxDEV(
  type: string | Function,
  props: Record<string, unknown>,
  key?: string
): JSX.Element;

export const Fragment: unique symbol;
