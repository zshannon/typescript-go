import TSGoBindings

func build() {
    var error: NSError?
    BridgeBuild("", &error)
}
