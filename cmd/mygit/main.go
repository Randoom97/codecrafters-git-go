package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/commands"
	"github.com/codecrafters-io/git-starter-go/cmd/mygit/utils"
)

func printCommandOutput(result string, err error) {
	if err != nil {
		utils.Error(err)
	}
	fmt.Print(result)
}

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		utils.Error(fmt.Errorf("usage: mygit <command> [<args>...]"))
	}

	switch command := os.Args[1]; command {
	case "init":
		printCommandOutput(commands.Initialize())
	case "cat-file":
		printCommandOutput(commands.CatFile())
	case "hash-object":
		printCommandOutput(commands.HashObject())
	default:
		utils.Error(fmt.Errorf("unknown command %s", command))
	}
}
