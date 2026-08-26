package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yext-eng/sqlparser"
)

type ParseErr struct {
	Idx  int
	Stmt string
	Err  error
}

func (e ParseErr) Error() string {
	return fmt.Sprintf("statement %d %q: %v", e.Idx, e.Stmt, e.Err)
}

var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()
}

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(fmt.Sprintf("failed to read stdin: %v", err))
	}

	stmts, err := sqlparser.SplitStatementToPieces(string(input))
	if err != nil {
		panic(fmt.Sprintf("failed to split statements: %v", err))
	}

	var errs []error
	for idx, stmt := range stmts {
		uncommented, _ := sqlparser.SplitMarginComments(stmt)
		if uncommented == "" {
			continue
		}
		if ast, err := sqlparser.Parse(stmt); err != nil {
			errs = append(errs, ParseErr{idx + 1, stmt, err})
		} else if verbose {
			fmt.Printf("%+v\n", ast)
		}
	}

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
