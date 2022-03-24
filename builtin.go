package main

import (
	"fmt"
	"strings"
)

// Builtin represents a function with a native (Go) implementation.
type Builtin struct {
	Name string
	Fn   func([]Sexpr) (Sexpr, error)
}

func (b Builtin) String() string {
	return fmt.Sprintf("<builtin: %s>", b.Name)
}

// Eval for builtin returns itself.
func (b Builtin) Eval(e *env) (Sexpr, error) {
	return b, nil
}

// Equal returns true if the receiver and the arg are both builtins and have the
// same name.
func (b Builtin) Equal(o Sexpr) bool {
	if o, ok := o.(Builtin); ok {
		return b.Name == o.Name
	}
	return false
}

var builtins = map[string]*Builtin{
	"+": {
		Name: "+",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) == 0 {
				return Num(0), nil
			}
			sum := Num(0)
			for _, arg := range args {
				n, ok := arg.(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", arg)
				}
				sum = sum.Add(n)
			}
			return sum, nil
		},
	},
	"-": {
		Name: "-",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("missing argument")
			}
			sum, ok := args[0].(Number)
			if !ok {
				return nil, fmt.Errorf("expected number, got '%s'", args[0])
			}
			if len(args) == 1 {
				return args[0].(Number).Neg(), nil
			}
			for _, arg := range args[1:] {
				n, ok := arg.(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", arg)
				}
				sum = sum.Sub(n)
			}
			return sum, nil
		},
	},
	"*": {
		Name: "*",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) == 0 {
				return Num(1), nil
			}
			prod := Num(1)
			for _, arg := range args {
				n, ok := arg.(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", arg)
				}
				prod = prod.Mul(n)
			}
			return prod, nil
		},
	},
	"/": {
		Name: "/",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("missing argument")
			}
			quot, ok := args[0].(Number)
			if !ok {
				return nil, fmt.Errorf("expected number, got '%s'", args[0])
			}
			for _, arg := range args[1:] {
				if arg.Equal(Num(0)) {
					return nil, fmt.Errorf("division by zero")
				}
				n, ok := arg.(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", arg)
				}
				quot = quot.Div(n)
			}
			return quot, nil
		},
	},
	"car": {
		Name: "car",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("missing argument")
			}
			carCons, ok := args[0].(*ConsCell)
			if !ok {
				return nil, fmt.Errorf("'%s' is not a list", args[0])
			}
			return carCons.car, nil
		},
	},
	"cdr": {
		Name: "cdr",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("missing argument")
			}
			cdrCons, ok := args[0].(*ConsCell)
			if !ok {
				return nil, fmt.Errorf("'%s' is not a list", args[0])
			}
			return cdrCons.cdr, nil
		},
	},
	"cons": {
		Name: "cons",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("missing argument")
			}
			return Cons(args[0], args[1].(*ConsCell)), nil
		},
	},
	"atom": {
		Name: "atom",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("missing argument")
			}
			_, ok := args[0].(Atom)
			if ok {
				return Atom{"t"}, nil
			}
			return Nil, nil
		},
	},
	"eq": {
		Name: "eq",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("missing argument")
			}
			for _, arg := range args[1:] {
				if !args[0].Equal(arg) {
					return Nil, nil
				}
			}
			return Atom{"t"}, nil
		},
	},
	"print": {
		Name: "print",
		Fn: func(args []Sexpr) (Sexpr, error) {
			strArgs := []string{}
			for _, arg := range args {
				strArgs = append(strArgs, arg.String())
			}
			fmt.Println(strings.Join(strArgs, " "))
			return Nil, nil
		},
	},
	"len": {
		Name: "len",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("len expects a single argument")
			}
			list, ok := args[0].(*ConsCell)
			if !ok {
				return nil, fmt.Errorf("'%s' is not a list", args[0])
			}
			count := 0
			for list != nil {
				count++
				list = list.cdr.(*ConsCell)
			}
			return Num(count), nil
		},
	},
	"split": {
		Name: "split",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("split expects a single argument")
			}
			switch s := args[0].(type) {
			case Atom:
				return listOfChars(s.String()), nil
			case Number:
				return listOfNums(s.String()), nil
			default:
				return nil, fmt.Errorf("split expects an atom or a number")
			}
		},
	},
	"apply": {
		Name: "apply",
		Fn: func(args []Sexpr) (Sexpr, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("apply expects exactly two arguments")
			}
			fnArgs := []Sexpr{}
			start := args[1]
			for start != Nil {
				cons, ok := start.(*ConsCell)
				if !ok {
					return nil, fmt.Errorf("'%s' is not a list", start)
				}
				fnArgs = append(fnArgs, cons.car)
				start = cons.cdr
			}
			return applyFn(args[0], fnArgs)
		},
	},
}

// listOfChars returns a list of single-character atoms from another, presumably
// longer atom; used by `split`
func listOfChars(s string) *ConsCell {
	if len(s) == 0 {
		return nil
	}
	return Cons(Atom{s[0:1]}, listOfChars(s[1:]))
}

// listOfNums returns a list of single-digit numbers from another, presumably
// longer number; used by `split`
func listOfNums(s string) *ConsCell {
	if len(s) == 0 {
		return nil
	}
	return Cons(Num(s[0:1]), listOfNums(s[1:]))
}
