package main

import (
	"github.com/RoGogDBD/metric-alerter/cmd/linter"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(linter.Analyzer)
}
