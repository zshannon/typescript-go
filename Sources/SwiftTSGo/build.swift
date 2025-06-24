import TSGoBindings

public func build(projectPath: String = ".") throws -> Bool {
    var error: NSError?
    let success = BridgeBuild(projectPath, &error)

    if let error = error {
        print("Build failed with error: \(error.localizedDescription)")
        throw error
    }

    return success
}
