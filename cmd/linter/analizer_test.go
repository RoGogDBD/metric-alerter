package linter_test

import (
	"testing"

	"github.com/RoGogDBD/metric-alerter/cmd/linter"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCheckCall(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, linter.Analyzer, "pkg1", "mainpkg")
}
