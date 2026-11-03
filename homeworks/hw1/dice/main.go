package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dicePattern = regexp.MustCompile(`^([0-9]+)d([0-9]+)$`)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("dice", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var count int
	var seed int64
	var sumOnly, verbose, help bool
	flags.IntVar(&count, "n", 1, "number of iterations")
	flags.IntVar(&count, "count", 1, "number of iterations")
	flags.Int64Var(&seed, "seed", time.Now().UnixNano(), "random seed")
	flags.BoolVar(&sumOnly, "s", false, "print only the sum")
	flags.BoolVar(&sumOnly, "sum", false, "print only the sum")
	flags.BoolVar(&verbose, "v", false, "print each die and averages")
	flags.BoolVar(&verbose, "verbose", false, "print each die and averages")
	flags.BoolVar(&help, "h", false, "show help")
	flags.BoolVar(&help, "help", false, "show help")

	flagArgs, diceArgs := splitArgs(args)
	if err := flags.Parse(flagArgs); err != nil {
		return err
	}
	if help {
		fmt.Fprintln(out, `Usage: dice [flags] NdM (1 <= N, M <= 100)
  -n, --count N  iterations (default 1)
      --seed N   random seed
  -s, --sum      print sums
  -v, --verbose  print dice and averages
  -h, --help     show help`)
		return nil
	}
	if sumOnly && verbose {
		return fmt.Errorf("--sum and --verbose cannot be used together")
	}
	if count < 1 {
		return fmt.Errorf("count must be positive")
	}
	if len(diceArgs) != 1 {
		return fmt.Errorf("expected exactly one dice argument in NdM format (try --help)")
	}
	match := dicePattern.FindStringSubmatch(diceArgs[0])
	if match == nil {
		return fmt.Errorf("invalid dice %q: expected NdM", diceArgs[0])
	}
	n, err := strconv.Atoi(match[1])
	if err != nil || n < 1 || n > 100 {
		return fmt.Errorf("number of dice must be between 1 and 100")
	}
	sides, err := strconv.Atoi(match[2])
	if err != nil || sides < 1 || sides > 100 {
		return fmt.Errorf("number of sides must be between 1 and 100")
	}

	r := rand.New(rand.NewSource(seed))
	if verbose {
		if count == 1 {
			fmt.Fprintf(out, "Rolling %dd%d:\n", n, sides)
		} else {
			fmt.Fprintf(out, "Rolling %dd%d (%d iterations):\n", n, sides, count)
		}
	}
	var total float64
	for iteration := 0; iteration < count; iteration++ {
		indent := ""
		if count > 1 {
			fmt.Fprintf(out, "Iteration %d:\n", iteration+1)
			indent = "  "
		}
		rolls := make([]string, n)
		sum := 0
		for die := 0; die < n; die++ {
			value := r.Intn(sides) + 1
			sum += value
			rolls[die] = strconv.Itoa(value)
			if verbose {
				fmt.Fprintf(out, "%sDice %d: %d\n", indent, die+1, value)
			}
		}
		total += float64(sum)
		switch {
		case verbose:
			fmt.Fprintf(out, "%sSum: %d\n%sAverage: %g\n", indent, sum, indent, float64(sum)/float64(n))
		case sumOnly:
			fmt.Fprintf(out, "%sSum: %d\n", indent, sum)
		case n == 1:
			fmt.Fprintf(out, "%sRolls: %s\n", indent, rolls[0])
		default:
			fmt.Fprintf(out, "%sRolls: %s=%d\n", indent, strings.Join(rolls, "+"), sum)
		}
	}
	if verbose && count > 1 {
		fmt.Fprintf(out, "Overall average: %g\n", total/float64(count)/float64(n))
	}
	return nil
}

func splitArgs(args []string) (flagArgs, diceArgs []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			diceArgs = append(diceArgs, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			diceArgs = append(diceArgs, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)
		switch strings.TrimLeft(arg, "-") {
		case "n", "count", "seed":
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		}
	}
	return flagArgs, diceArgs
}
