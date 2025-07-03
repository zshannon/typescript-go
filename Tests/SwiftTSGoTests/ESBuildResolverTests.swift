import Foundation
import Testing

@testable import SwiftTSGo

@Suite(.serialized)
struct ESBuildResolverTests {
    
    // Helper function to convert Data to String
    private func dataToString(_ data: Data) -> String {
        return String(data: data, encoding: .utf8) ?? ""
    }
    
    @Test func basicBundleTest() async throws {
        let files = [
            "/src/index.js": """
            console.log('Hello, World!');
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Check that output contains our code
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        
        #expect(dataToString(jsOutput!.contents).contains("Hello, World!"))
    }
    
    @Test func relativeImportsTest() async throws {
        let files = [
            "/src/index.js": """
            import { greet } from './utils.js';
            console.log(greet('World'));
            """,
            "/src/utils.js": """
            export function greet(name) {
                return `Hello, ${name}!`;
            }
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Check that both files were bundled
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("greet"))
        #expect(dataToString(jsOutput!.contents).contains("World"))
    }
    
    @Test func typeScriptSupportTest() async throws {
        let files = [
            "/src/index.ts": """
            interface User {
                name: string;
                age: number;
            }
            
            function greet(user: User): string {
                return `Hello, ${user.name}! You are ${user.age} years old.`;
            }
            
            const user: User = { name: 'Alice', age: 30 };
            console.log(greet(user));
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.ts"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Check that TypeScript was compiled to JavaScript
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("Alice"))
        #expect(dataToString(jsOutput!.contents).contains("30"))
        // TypeScript types should be stripped
        #expect(!dataToString(jsOutput!.contents).contains("interface"))
        #expect(!dataToString(jsOutput!.contents).contains("User"))
    }
    
    @Test func jsxSupportTest() async throws {
        let files = [
            "/src/App.jsx": """
            function App() {
                return <div>Hello, React!</div>;
            }
            
            export default App;
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/App.jsx"]
        options.format = .esmodule
        options.jsx = .transform
        options.jsxFactory = "React.createElement"
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Check that JSX was transformed
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("React.createElement"))
        #expect(dataToString(jsOutput!.contents).contains("Hello, React!"))
    }
    
    @Test func multipleEntryPointsTest() async throws {
        let files = [
            "/src/page1.js": """
            console.log('Page 1');
            """,
            "/src/page2.js": """
            console.log('Page 2');
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/page1.js", "/src/page2.js"]
        options.format = .esmodule
        options.outdir = "/out"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(result!.outputFiles.count >= 2)
        
        // Should have output for both entry points
        let page1Output = result!.outputFiles.first { dataToString($0.contents).contains("Page 1") }
        let page2Output = result!.outputFiles.first { dataToString($0.contents).contains("Page 2") }
        #expect(page1Output != nil)
        #expect(page2Output != nil)
    }
    
    @Test func minificationTest() async throws {
        let files = [
            "/src/index.js": """
            function veryLongFunctionName(veryLongParameterName) {
                const veryLongVariableName = 'Hello, World!';
                console.log(veryLongVariableName + ' ' + veryLongParameterName);
                return veryLongVariableName;
            }
            
            veryLongFunctionName('Test');
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.minifyWhitespace = true
        options.minifyIdentifiers = true
        options.minifySyntax = true
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        
        // Check that code was minified (shorter variable names, no extra whitespace)
        let minifiedCode = dataToString(jsOutput!.contents)
        #expect(!minifiedCode.contains("veryLongFunctionName"))
        #expect(!minifiedCode.contains("veryLongParameterName"))
        #expect(minifiedCode.count < 200) // Original is much longer
    }
    
    @Test func cssImportTest() async throws {
        let files = [
            "/src/index.js": """
            import './styles.css';
            console.log('Styled app');
            """,
            "/src/styles.css": """
            body {
                background-color: #f0f0f0;
                font-family: Arial, sans-serif;
            }
            
            .container {
                max-width: 800px;
                margin: 0 auto;
            }
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Should have both JS and CSS outputs
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        let cssOutput = result!.outputFiles.first { $0.path.hasSuffix(".css") }
        
        #expect(jsOutput != nil)
        #expect(cssOutput != nil)
        #expect(dataToString(cssOutput!.contents).contains("background-color"))
        #expect(dataToString(cssOutput!.contents).contains("font-family"))
    }
    
    @Test func errorHandlingTest() async throws {
        let files = [
            "/src/index.js": """
            import { nonExistentFunction } from './missing-file.js';
            nonExistentFunction();
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(!result!.errors.isEmpty)
        
        // Should have error about missing file
        let error = result!.errors.first
        #expect(error != nil)
        #expect(error!.text.contains("missing-file") || error!.text.contains("resolve"))
    }
    
    @Test func nestedDirectoriesTest() async throws {
        let files = [
            "/src/components/Button.jsx": """
            export function Button({ children, onClick }) {
                return <button onClick={onClick}>{children}</button>;
            }
            """,
            "/src/utils/helpers.js": """
            export function formatDate(date) {
                return date.toLocaleDateString();
            }
            """,
            "/src/App.jsx": """
            import { Button } from './components/Button.jsx';
            import { formatDate } from './utils/helpers.js';
            
            function App() {
                const handleClick = () => console.log('Clicked!');
                return (
                    <div>
                        <h1>Today is {formatDate(new Date())}</h1>
                        <Button onClick={handleClick}>Click me</Button>
                    </div>
                );
            }
            
            export default App;
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/App.jsx"]
        options.format = .esmodule
        options.jsx = .transform
        options.jsxFactory = "React.createElement"
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("formatDate"))
        #expect(dataToString(jsOutput!.contents).contains("Button"))
        #expect(dataToString(jsOutput!.contents).contains("Click me"))
    }
    
    @Test func resolverBasedBuildTest() async throws {
        // Test the resolver-based function directly
        let resolver: @Sendable (String) async throws -> FileResolver? = { path in
            switch path {
            case "/src/index.js":
                return .file("""
                import { multiply } from './math.js';
                console.log('2 * 3 =', multiply(2, 3));
                """)
            case "/src/math.js":
                return .file("""
                export function multiply(a, b) {
                    return a * b;
                }
                """)
            case "/src":
                return .directory(["index.js", "math.js"])
            default:
                return nil
            }
        }
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(options: options, resolver: resolver)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("multiply"))
        #expect(dataToString(jsOutput!.contents).contains("2 * 3"))
    }
    
    @Test func jsonImportTest() async throws {
        let files = [
            "/src/index.js": """
            import config from './config.json';
            console.log('App name:', config.name);
            console.log('Version:', config.version);
            """,
            "/src/config.json": """
            {
                "name": "My App",
                "version": "1.0.0",
                "features": ["auth", "dashboard"]
            }
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .esmodule
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("My App"))
        #expect(dataToString(jsOutput!.contents).contains("1.0.0"))
    }
    
    @Test func sourceMapsTest() async throws {
        let files = [
            "/src/index.ts": """
            interface Config {
                apiUrl: string;
                timeout: number;
            }
            
            const config: Config = {
                apiUrl: 'https://api.example.com',
                timeout: 5000
            };
            
            console.log('API URL:', config.apiUrl);
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.ts"]
        options.format = .esmodule
        options.sourcemap = .external
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        // Should have both JS and source map files
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        let mapOutput = result!.outputFiles.first { $0.path.hasSuffix(".js.map") }
        
        #expect(jsOutput != nil)
        #expect(mapOutput != nil)
        #expect(dataToString(mapOutput!.contents).contains("sourcesContent"))
        #expect(dataToString(mapOutput!.contents).contains("index.ts"))
    }
    
    @Test func commonJSFormatTest() async throws {
        let files = [
            "/src/index.js": """
            const { helper } = require('./helper.js');
            console.log(helper('CommonJS'));
            """,
            "/src/helper.js": """
            function helper(message) {
                return `Helper says: ${message}`;
            }
            
            module.exports = { helper };
            """,
        ]
        
        var options = ESBuildBuildOptions()
        options.bundle = true
        options.write = false
        options.entryPoints = ["/src/index.js"]
        options.format = .commonjs
        options.platform = .node
        options.outfile = "/out/bundle.js"
        
        let result = try await esbuildBuild(files: files, options: options)
        
        #expect(result != nil)
        #expect(result!.errors.isEmpty)
        #expect(!result!.outputFiles.isEmpty)
        
        let jsOutput = result!.outputFiles.first { $0.path.hasSuffix(".js") }
        #expect(jsOutput != nil)
        #expect(dataToString(jsOutput!.contents).contains("Helper says"))
        #expect(dataToString(jsOutput!.contents).contains("CommonJS"))
    }
}