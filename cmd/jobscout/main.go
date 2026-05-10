package main

import (
	"os"

	"github.com/wallentx/jobscout/internal/jobscout"
)

func main() {
	os.Exit(jobscout.Run(os.Args))
}
