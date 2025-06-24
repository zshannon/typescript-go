import TSGoBindings

public func build(projectPath: String = ".") -> Bool {
    var error: NSError?
    let success = BridgeBuild(projectPath, &error)

    if let error = error {
        print("Build failed with error: \(error.localizedDescription)")
        return false
    }

    return success
}
