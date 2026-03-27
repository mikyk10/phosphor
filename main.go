package main

import (
	"os"

	"github.com/mikyk10/phosphor/cmd"
)

func main() {
	cmd.Execute(os.Args[1:])
}
