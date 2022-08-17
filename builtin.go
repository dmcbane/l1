package main

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Builtin represents a function with a native (Go) implementation.
type Builtin struct {
	Name string
	Fn   func([]Sexpr, *env) (Sexpr, error)
	// Function must take at least this many arguments
	FixedArity int
	// Function can take more arguments
	NAry      bool
	Docstring string
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

func lambdaDescription(l lambdaFn) string {
	if l.doc == Nil {
		return "UNDOCUMENTED"
	}
	carDoc := l.doc.car.String()
	shortDoc := carDoc[1 : len(carDoc)-1]
	return shortDoc
}

func lambdaDocString(name string, e *env) string {
	expr, _ := e.Lookup(name)
	l, ok := expr.(*lambdaFn)
	if !ok {
		panic("should have gotten a lambda here")
	}
	isMultiArity := " "
	if l.restArg != "" {
		isMultiArity = "+"
	}
	argstr := fmt.Sprintf("%d%s", len(l.args), isMultiArity)
	return fmt.Sprintf("%10s %5s     %s\n",
		name,
		argstr,
		capitalize(lambdaDescription(*l)))
}

func doHelp(out io.Writer, e *env) {
	fmt.Fprintln(out, "Builtins and Special Forms:")
	fmt.Fprintln(out, "      Name  Arity    Description")
	type fnDoc struct {
		name      string
		farity    int
		ismulti   bool
		doc       string
		isSpecial bool
	}
	forms := []fnDoc{
		{"cond", 0, true, "Conditional branching", true},
		{"def", 2, false, "Set a value", true},
		{"defn", 2, true, "Create and name a function", true},
		{"errors", 1, true, "Error checking (for tests)", true},
		{"lambda", 1, true, "Create a function", true},
		{"let", 1, true, "Create a local scope", true},
		{"quote", 1, false, "Quote an expression", true},
		{"and", 0, true, "Boolean and", true},
		{"or", 0, true, "Boolean or", true},
	}
	for _, builtin := range builtins {
		forms = append(
			forms,
			fnDoc{
				builtin.Name,
				builtin.FixedArity,
				builtin.NAry,
				builtin.Docstring,
				false})
	}
	// sort by name
	sort.Slice(forms, func(i, j int) bool {
		return forms[i].name < forms[j].name
	})
	for _, form := range forms {
		special := ""
		if form.isSpecial {
			special = "SPECIAL FORM: "
		}
		isMultiArity := " "
		if form.ismulti {
			isMultiArity = "+"
		}
		argstr := fmt.Sprintf("%d%s", form.farity, isMultiArity)
		fmt.Fprintf(
			out,
			"%10s %5s     %s%s\n",
			form.name,
			argstr,
			special,
			form.doc)
	}
	lambdaNames := []string{}
	for _, name := range EnvKeys(e) {
		expr, _ := e.Lookup(name)
		if _, ok := expr.(*lambdaFn); ok {
			lambdaNames = append(lambdaNames, name)
		}
	}
	sort.Slice(lambdaNames, func(i, j int) bool {
		return lambdaNames[i] < lambdaNames[j]
	})

	fmt.Fprint(out, "\n\nOther available functions:\n\n")
	for _, lambdaName := range lambdaNames {
		fmt.Fprint(out, lambdaDocString(lambdaName, e))
	}
}

func compareMultipleNums(cmp func(a, b Number) bool, args []Sexpr) (Sexpr, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("missing argument")
	}
	first, ok := args[0].(Number)
	if !ok {
		return nil, fmt.Errorf("'%s' is not a number", args[0])
	}
	last := first
	for i := 1; i < len(args); i++ {
		num, ok := args[i].(Number)
		if !ok {
			return nil, fmt.Errorf("'%s' is not a number", args[i])
		}
		if !cmp(num, last) {
			return Nil, nil
		}
		last = num
	}
	return True, nil
}

// moving `builtins` into `init` avoids initialization loop for doHelp:
var builtins map[string]*Builtin

func init() {
	builtins = map[string]*Builtin{
		"+": {
			Name:       "+",
			Docstring:  "Add 0 or more numbers",
			FixedArity: 0,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
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
			Name:       "-",
			Docstring:  "Subtract 0 or more numbers from the first argument",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
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
			Name:       "*",
			Docstring:  "Multiply 0 or more numbers",
			FixedArity: 0,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
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
			Name:       "/",
			Docstring:  "Divide the first argument by the rest",
			FixedArity: 2,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) < 1 {
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
		"=": {
			Name:       "=",
			Docstring:  "Return t if the arguments are equal, () otherwise",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("missing argument")
				}
				for _, arg := range args[1:] {
					if !args[0].Equal(arg) {
						return Nil, nil
					}
				}
				return True, nil
			},
		},
		"rem": {
			Name:       "rem",
			Docstring:  "Return remainder when second arg divides first",
			FixedArity: 2,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("rem requires two arguments")
				}
				n1, ok := args[0].(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", args[0])
				}
				n2, ok := args[1].(Number)
				if !ok {
					return nil, fmt.Errorf("expected number, got '%s'", args[1])
				}
				if n2.Equal(Num(0)) {
					return nil, fmt.Errorf("division by zero")
				}
				return n1.Rem(n2), nil
			},
		},
		"<": {
			Name:       "<",
			Docstring:  "Return t if the arguments are in strictly increasing order, () otherwise",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				return compareMultipleNums(func(a, b Number) bool {
					return b.Less(a)
				}, args)
			},
		},
		"<=": {
			Name:       "<=",
			Docstring:  "Return t if the arguments are in increasing (or qual) order, () otherwise",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				return compareMultipleNums(func(a, b Number) bool {
					return b.LessEqual(a)
				}, args)
			},
		},
		">": {
			Name:       ">",
			Docstring:  "Return t if the arguments are in strictly decreasing order, () otherwise",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				return compareMultipleNums(func(a, b Number) bool {
					return b.Greater(a)
				}, args)
			},
		},
		">=": {
			Name:       ">=",
			Docstring:  "Return t if the arguments are in decreasing (or equal) order, () otherwise",
			FixedArity: 1,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				return compareMultipleNums(func(a, b Number) bool {
					return b.GreaterEqual(a)
				}, args)
			},
		},
		"apply": {
			Name:       "apply",
			Docstring:  "Apply a function to a list of arguments",
			FixedArity: 2,
			NAry:       false,
			Fn: func(args []Sexpr, env *env) (Sexpr, error) {
				if len(args) < 2 {
					return nil, fmt.Errorf("not enough arguments")
				}
				l := len(args)
				var fnArgs []Sexpr
				// Support (apply f a b l) where l is a list and a, b are scalars:
				singleArgs := args[1 : l-1]
				fnArgs, err := consToExprs(args[l-1])
				if err != nil {
					return nil, err
				}
				fnArgs = append(singleArgs, fnArgs...)

				// Note: what follows is very similar to the function evaluation
				// logic in eval(), but TCO (goto) there makes it hard to DRY out with
				// respect to what follows.

				evalCar := args[0]
				// User-defined functions:
				lambda, ok := evalCar.(*lambdaFn)
				if ok {
					newEnv := mkEnv(lambda.env)
					err := setLambdaArgsInEnv(&newEnv, lambda, fnArgs)
					if err != nil {
						return nil, err
					}
					var ret Sexpr = Nil
					bodyExpr := lambda.body
					for {
						if bodyExpr == Nil {
							return ret, nil
						}
						ret, err = eval(bodyExpr.car, &newEnv)
						if err != nil {
							return nil, err
						}
						bodyExpr = bodyExpr.cdr.(*ConsCell)
					}
				}
				// Built-in functions:
				builtin, ok := evalCar.(*Builtin)
				if !ok {
					return nil, fmt.Errorf("%s is not a function", evalCar)
				}
				biResult, err := builtin.Fn(fnArgs, env)
				if err != nil {
					return nil, err
				}
				return biResult, nil
			},
		},
		"atom?": {
			Name:       "atom?",
			Docstring:  "Return t if the argument is an atom, () otherwise",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("atom? expects a single argument")
				}
				if _, ok := args[0].(Atom); ok {
					return True, nil
				}
				return Nil, nil
			},
		},
		"bang": {
			Name:       "bang",
			Docstring:  "Return a new atom with exclamation point added",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				a, ok := args[0].(Atom)
				if !ok {
					return nil, fmt.Errorf("expected atom, got '%s'", args[0])
				}
				return Atom{a.s + "!"}, nil
			},
		},
		"body": {
			Name:       "body",
			Docstring:  "Return the body of a lambda function",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				l, ok := args[0].(*lambdaFn)
				if !ok {
					return nil, fmt.Errorf("expected lambda function, got '%s'", args[0])
				}
				return l.body, nil
			},
		},
		"car": {
			Name:       "car",
			Docstring:  "Return the first element of a list",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				carCons, ok := args[0].(*ConsCell)
				if !ok {
					return nil, fmt.Errorf("'%s' is not a list", args[0])
				}
				if carCons == Nil {
					return Nil, nil
				}
				return carCons.car, nil
			},
		},
		"cdr": {
			Name:       "cdr",
			Docstring:  "Return a list with the first element removed",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				cdrCons, ok := args[0].(*ConsCell)
				if !ok {
					return nil, fmt.Errorf("'%s' is not a list", args[0])
				}
				if cdrCons == Nil {
					return Nil, nil
				}
				return cdrCons.cdr, nil
			},
		},
		"comma": {
			Name:       "comma",
			Docstring:  "Return a new atom with a comma at the end",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("comma requires one argument")
				}
				a, ok := args[0].(Atom)
				if !ok {
					return nil, fmt.Errorf("expected atom, got '%s'", args[0])
				}
				return Atom{a.s + ","}, nil
			},
		},
		"cons": {
			Name:       "cons",
			Docstring:  "Add an element to the front of a (possibly empty) list",
			FixedArity: 2,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 2 {
					return nil, fmt.Errorf("missing argument")
				}
				return Cons(args[0], args[1]), nil
			},
		},
		"doc": {
			Name:       "doc",
			Docstring:  "Return the doclist for a function",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				lambda, ok := args[0].(*lambdaFn)
				if !ok {
					return nil, fmt.Errorf("expected function, got '%s'", args[0])
				}
				return lambda.doc, nil
			},
		},
		"downcase": {
			Name:       "downcase",
			Docstring:  "Return a new atom with all characters in lower case",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("downcase requires one argument")
				}
				a, ok := args[0].(Atom)
				if !ok {
					return nil, fmt.Errorf("expected atom, got '%s'", args[0])
				}
				return Atom{strings.ToLower(a.s)}, nil
			},
		},
		"fuse": {
			Name:       "fuse",
			Docstring:  "Fuse a list of numbers or atoms into a single atom",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("fuse expects a single argument")
				}
				if args[0] == Nil {
					return Nil, nil
				}
				switch s := args[0].(type) {
				case *ConsCell:
					cons := s
					var str string
					for cons != nil {
						this := cons.car.String()
						str += this
						if cons.cdr == nil {
							break
						}
						cons = cons.cdr.(*ConsCell)
					}
					// if first rune is a digit, return a Number
					firstRune, _ := utf8.DecodeRuneInString(str)
					if unicode.IsDigit(firstRune) {
						return Num(str), nil
					}
					return Atom{str}, nil
				default:
					return nil, fmt.Errorf("fuse expects a list")
				}
			},
		},
		"help": {
			Name:       "help",
			Docstring:  "Print this message",
			FixedArity: 0,
			NAry:       false,
			Fn: func(args []Sexpr, e *env) (Sexpr, error) {
				doHelp(os.Stdout, e)
				return Nil, nil
			},
		},
		"is": {
			Name:       "is",
			Docstring:  "Assert that the argument is truthy (not ())",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				if args[0] == Nil {
					return nil, fmt.Errorf("'is' assertion failed")
				}
				return args[0], nil
			},
		},
		"len": {
			Name:       "len",
			Docstring:  "Return the length of a list",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
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
		"list": {
			Name:       "list",
			Docstring:  "Return a list of the given arguments",
			FixedArity: 0,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				return mkListAsConsWithCdr(args, Nil), nil
			},
		},
		"list?": {
			Name:       "list?",
			Docstring:  "Return t if the argument is a list, () otherwise",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("list? expects a single argument")
				}
				if _, ok := args[0].(*ConsCell); ok {
					return True, nil
				}
				return Nil, nil
			},
		},
		"not": {
			Name:       "not",
			Docstring:  "Return t if the argument is nil, () otherwise",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("not expects a single argument")
				}
				if args[0] == Nil {
					return True, nil
				}
				return Nil, nil
			},
		},
		"period": {
			Name:       "period",
			Docstring:  "Return a new atom with a period added to the end",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("period requires one argument")
				}
				a, ok := args[0].(Atom)
				if !ok {
					return nil, fmt.Errorf("expected atom, got '%s'", args[0])
				}
				return Atom{a.s + "."}, nil
			},
		},
		"print": {
			Name:       "print",
			Docstring:  "Print the arguments",
			FixedArity: 0,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				strArgs := []string{}
				for _, arg := range args {
					strArgs = append(strArgs, arg.String())
				}
				fmt.Println(strings.Join(strArgs, " "))
				return Nil, nil
			},
		},
		"printl": {
			Name:       "printl",
			Docstring:  "Print a list argument, without parentheses",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("missing argument")
				}
				fmt.Println(args[0].String()[1 : len(args[0].String())-1])
				return Nil, nil
			},
		},
		"split": {
			Name:       "split",
			Docstring:  "Split an atom or number into a list of single-digit numbers or single-character atoms",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("split expects a single argument")
				}
				switch s := args[0].(type) {
				case Atom:
					return listOfChars(s.String()), nil
				case Number:
					return listOfNums(s.String())
				default:
					return nil, fmt.Errorf("split expects an atom or a number")
				}
			},
		},
		"number?": {
			Name:       "number?",
			Docstring:  "Return true if the argument is a number, else ()",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("number? expects a single argument")
				}
				_, ok := args[0].(Number)
				if ok {
					return True, nil
				}
				return Nil, nil
			},
		},
		"pos?": {
			Name:       "pos?",
			Docstring:  "Return true if the (numeric) argument is positive, else ()",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("pos? expects a single argument")
				}
				num, ok := args[0].(Number)
				if !ok {
					return nil, fmt.Errorf("'%s' is not a number", args[0])
				}
				if num.Greater(Num(0)) {
					return True, nil
				}
				return Nil, nil
			},
		},
		"randint": {
			Name:       "randint",
			Docstring:  "Return a random integer between 0 and the argument minus 1",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("randint expects a single argument")
				}
				num, ok := args[0].(Number)
				if !ok {
					return nil, fmt.Errorf("'%s' is not a number", args[0])
				}
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				return Num(r.Intn(int(num.bi.Uint64()))), nil
			},
		},
		"test": {
			Name:       "test",
			Docstring:  "Establish a testing block (return last expression)",
			FixedArity: 0,
			NAry:       true,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) == 0 {
					return Nil, nil
				}
				fmt.Printf("TEST %s ", args[0].String())
				for range args[1:] {
					fmt.Print(".")
				}
				fmt.Println("✓")
				return args[len(args)-1], nil
			},
		},
		"upcase": {
			Name:       "upcase",
			Docstring:  "Return the uppercase version of the given atom",
			FixedArity: 1,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("upcase expects a single argument")
				}
				a, ok := args[0].(Atom)
				if !ok {
					return nil, fmt.Errorf("upcase expects an atom")
				}
				return Atom{strings.ToUpper(a.s)}, nil
			},
		},
		"version": {
			Name:       "version",
			Docstring:  "Return the version of the interpreter",
			FixedArity: 0,
			NAry:       false,
			Fn: func(args []Sexpr, _ *env) (Sexpr, error) {
				versionSexprs := semverAsExprs(version)
				return mkListAsConsWithCdr(versionSexprs, Nil), nil
			},
		},
	}
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
// longer number; used by `split`; if the input represents a negative number,
// the first digit is negative:
func listOfNums(s string) (*ConsCell, error) {
	if len(s) == 0 {
		return nil, nil
	}
	if s[0] == '-' {
		if len(s) < 2 {
			return nil, fmt.Errorf("unexpected end of input")
		}
		lon, err := listOfNums(s[2:])
		if err != nil {
			return nil, err
		}
		return Cons(Num(s[0:2]), lon), nil
	}
	lon, err := listOfNums(s[1:])
	if err != nil {
		return nil, err
	}
	return Cons(Num(s[0:1]), lon), nil
}

func semverAsExprs(semver string) []Sexpr {
	reg := regexp.MustCompile(`(?:^v)?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-(dirty))?`)
	matches := reg.FindStringSubmatch(semver)
	if len(matches) == 0 {
		return nil
	}
	list := []Sexpr{}
	for _, m := range matches[1:] {
		if len(m) == 0 {
			continue
		}
		if m == "dirty" {
			list = append(list, Atom{"dirty"})
		} else {
			list = append(list, Num(m))
		}
	}
	return list
}
