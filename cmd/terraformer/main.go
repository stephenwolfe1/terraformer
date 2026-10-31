package main

import (
	"os"

	"github.com/stephenwolfe1/terraformer/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
