/*
Copyright © 2022 Pavel Tisnovsky

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"

	"go/scanner"
	"go/token"
)

// expression that we need to evaluate
const source = `
confidence > 1 && (severity-1) >= 2 && priority < severity*2-1
`

func main() {
	// scanner object (lexer)
	var s scanner.Scanner

	// structure that represents set of source file(s)
	fset := token.NewFileSet()

	// info about source file
	file := fset.AddFile("", fset.Base(), len(source))

	// initialize the scanner
	s.Init(file, []byte(source), nil, scanner.ScanComments)

	// transform input expression into postfix notation
	postfixExpression := toRPN(s)

	// fmt.Println(postfixExpression)

	// values that can be used in expression
	values := make(map[string]int)
	values["confidence"] = 2
	values["severity"] = 3
	values["priority"] = 1

	// evaluate the expression represented in postfix notation
	stack, err := evaluate(postfixExpression, values)
	if err != nil {
		fmt.Println(err)
		return
	}

	// print operand stack
	printStack(stack)
}
