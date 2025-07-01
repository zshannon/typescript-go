// Detailed information about a TypeScript diagnostic
public struct DiagnosticInfo: Sendable {
    public let code: Int
    public let category: String
    public let message: String
    public let file: String
    public let line: Int
    public let column: Int
    public let length: Int

    public init(
        code: Int, category: String, message: String, file: String = "", line: Int = 0,
        column: Int = 0, length: Int = 0
    ) {
        self.code = code
        self.category = category
        self.message = message
        self.file = file
        self.line = line
        self.column = column
        self.length = length
    }
}
