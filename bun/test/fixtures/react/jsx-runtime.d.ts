import * as React from './';

export namespace JSX {
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

  interface ElementChildrenAttribute {
    children: {};
  }

  interface IntrinsicAttributes {
    key?: string | number;
  }
}

export function jsx(type: any, props: any, key?: string): JSX.Element;
export function jsxs(type: any, props: any, key?: string): JSX.Element;
export function jsxDEV(type: any, props: any, key?: string): JSX.Element;
export { JSX };
