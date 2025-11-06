package main

import (
	"github.com/yayoc/testsnake"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(testsnake.Analyzer) }
