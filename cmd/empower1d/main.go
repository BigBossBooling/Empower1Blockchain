package main

import (
	"log"

	"github.com/empower1/empower1/internal/core"
	"github.com/empower1/empower1/cmd/empower1d/cli"
)

func main() {
	bc := core.NewBlockchain()
	defer bc.Db().Close()

	cli := cli.NewCLI(bc)

	err := cli.Execute()
	if err != nil {
		log.Panic(err)
	}
}
