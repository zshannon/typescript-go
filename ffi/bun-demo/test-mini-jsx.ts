import tsgo from "./tsgo";

// Using UNIQUE element names that DON'T exist in react-dom!
// zframe, zheading, zstack, ztext, zbutton are custom elements
// If this passes, it PROVES jsxImportSource is actually working
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

const result = tsgo.typecheckWithOptions(code, "/project/App.tsx", {
  jsx: "react-jsx",
  jsxImportSource: "@mini/jsx-runtime",
  target: "ES2022",
  module: "ESNext",
  moduleResolution: "Bundler",
  strict: true,
  skipLibCheck: true,
  lib: ["ES2022", "DOM"],
});

console.log("Custom JSX Runtime (@mini/jsx-runtime):");
console.log(`  Success: ${result.success}`);
console.log(`  Duration: ${result.duration_ms.toFixed(2)}ms`);
console.log(`  Diagnostics: ${result.diagnostics?.length ?? 0}`);
if (result.diagnostics?.length) {
  for (const d of result.diagnostics) {
    console.log(`  - ${d.message}`);
  }
}
