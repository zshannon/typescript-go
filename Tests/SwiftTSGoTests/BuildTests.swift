import Foundation
import Testing

@testable import SwiftTSGo

@Test func helloWorldTest() throws {

    // Get the path to the test project directory
    let testBundle = Bundle.module
    let testProjectPath = testBundle.path(forResource: "test-hello", ofType: nil)!

    let success = build(projectPath: testProjectPath)
    #expect(success == true)

    let distPath = URL(fileURLWithPath: testProjectPath).appendingPathComponent("dist/hello.js")
        .path
    let contents = try String(contentsOfFile: distPath)
    #expect(
        contents == """
            "use strict";
            Object.defineProperty(exports, "__esModule", { value: true });
            exports.greet = greet;
            function greet(name) {
                return `Hello, ${name}!`;
            }
            const message = greet("World");
            console.log(message);

            """)
}
