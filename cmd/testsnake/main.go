package main

import (
	"github.com/yayoc/test-snake"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(testsnake.Analyzer) }
