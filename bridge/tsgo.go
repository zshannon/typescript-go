// bridge/tsgo.go
package bridge

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/execute"
)

func Build(configPath string) error {
	exit := execute.CommandLine(nil, nil, []string{"build", "-p", configPath})
	if exit != 0 {
		return fmt.Errorf("tsgo build failed with exit code %d", exit)
	}
	return nil
}
