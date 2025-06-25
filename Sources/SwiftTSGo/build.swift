import TSGoBindings

public func build(projectPath: String = ".") throws -> Bool {
    var error: NSError?
    let success = BridgeBuild(projectPath, &error)

    if let error = error {
        throw error
    }

    return success
}
