package main

import (
	"os"

	"github.com/mikyk10/wisp-ai/cmd"
)

func main() {
	cmd.Execute(os.Args[1:])
}
