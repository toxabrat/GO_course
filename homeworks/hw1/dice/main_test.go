package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDiceOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"one die", []string{"--seed", "42", "1d1"}, "Rolls: 1\n"},
		{"seeded rolls", []string{"--seed", "42", "4d20"}, "Rolls: 6+8+9+11=34\n"},
		{"seeded sum", []string{"--seed=42", "--sum", "4d20"}, "Sum: 34\n"},
		{"seeded verbose", []string{"-v", "--seed", "42", "4d20"}, "Rolling 4d20:\nDice 1: 6\nDice 2: 8\nDice 3: 9\nDice 4: 11\nSum: 34\nAverage: 8.5\n"},
		{"flags after dice", []string{"2d1", "-n", "2", "-s"}, "Iteration 1:\n  Sum: 2\nIteration 2:\n  Sum: 2\n"},
		{"iterations preserve RNG", []string{"--seed", "42", "--count", "3", "2d10"}, "Iteration 1:\n  Rolls: 6+8=14\nIteration 2:\n  Rolls: 9+1=10\nIteration 3:\n  Rolls: 4+6=10\n"},
		{"overall average", []string{"-n", "2", "-v", "2d1"}, "Rolling 2d1 (2 iterations):\nIteration 1:\n  Dice 1: 1\n  Dice 2: 1\n  Sum: 2\n  Average: 1\nIteration 2:\n  Dice 1: 1\n  Dice 2: 1\n  Sum: 2\n  Average: 1\nOverall average: 1\n"},
		{"end of flags", []string{"--seed=0", "--", "1d1"}, "Rolls: 1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tt.args, &out); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiceInvalidArguments(t *testing.T) {
	tests := [][]string{
		{}, {"d6"}, {"2D6"}, {"-1d6"}, {"+1d6"}, {"0d6"}, {"1d0"},
		{"101d6"}, {"1d101"}, {"1d6d6"}, {"1d6", "2d6"},
		{"99999999999999999999999d6"}, {"1d99999999999999999999999"},
		{"-n", "0", "1d6"}, {"--count=-1", "1d6"}, {"-n", "abc", "1d6"},
		{"--seed", "abc", "1d6"}, {"--seed"}, {"--unknown", "1d6"},
		{"-s", "--verbose", "1d6"}, {"--sum", "-v", "1d6"}, {"--", "1d6", "-s"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if err := run(args, &bytes.Buffer{}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestDiceHelpAndLimits(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		var out bytes.Buffer
		if err := run([]string{arg}, &out); err != nil || !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("help: %q, %v", out.String(), err)
		}
	}
	var out bytes.Buffer
	if err := run([]string{"--seed=0", "100d100"}, &out); err != nil {
		t.Fatal(err)
	}
	first := out.String()
	out.Reset()
	if err := run([]string{"--seed=0", "100d100"}, &out); err != nil || out.String() != first {
		t.Fatalf("seed 0 is not reproducible: %v", err)
	}
}
