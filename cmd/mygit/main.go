package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/commands"
)

func printCommandOutput(result string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	fmt.Print(result)
}

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		printCommandOutput(commands.Initialize())
	case "cat-file":
		printCommandOutput(commands.CatFile())
	case "hash-object":
		printCommandOutput(commands.HashObject())
	case "ls-tree":
		printCommandOutput(commands.LsTree())
	case "write-tree":
		printCommandOutput(commands.WriteTree())
	case "commit-tree":
		printCommandOutput(commands.CommitTree())
	default:
		fmt.Fprintf(os.Stderr, "unknown command %s\n", command)
		os.Exit(1)
	}
}
