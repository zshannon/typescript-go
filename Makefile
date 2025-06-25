build:
	gomobile bind -target=ios,iossimulator,macos -o Sources/TSGoBindings.xcframework github.com/microsoft/typescript-go/bridge

setup:
	go get -d golang.org/x/mobile/cmd/gomobile
	go install golang.org/x/mobile/cmd/gomobile
	gomobile init

test:
	@cd bridge && go test
