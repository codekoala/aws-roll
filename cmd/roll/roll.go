package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/codekoala/aws-roll"
	"github.com/codekoala/aws-roll/version"
)

func init() {
	var showVersion bool

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `aws-roll %s

aws-roll is a helper to remove an EC2 instance from an Elastic Load Balancer
while a command (such as a deployment script) is running, adding the instance
back into the ELB when the command finishes.

Usage:
	%s CMD
	%s "command-to-run && next command"

Options:
`, version.Version, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	flag.BoolVar(&showVersion, "v", false, "show version information")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Detailed())
		os.Exit(0)
	}

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	roll.Roll(flag.Arg(0))
}
