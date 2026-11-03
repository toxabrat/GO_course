package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	handlererrors "github.com/tcarzverey/course-go-python/homeworks/hw1/handler/errors"
)

type sequenceDB struct {
	errors []error
	calls  int
}

func (d *sequenceDB) UpdateBalance(context.Context, int64, int64) error {
	index := min(d.calls, len(d.errors)-1)
	d.calls++
	return d.errors[index]
}

func TestRetryBudget(t *testing.T) {
	first := fmt.Errorf("first: %w", handlererrors.NewRetryableError(2))
	last := fmt.Errorf("last: %w", handlererrors.NewRetryableError(100))
	zero := handlererrors.NewRetryableError(0)
	negative := handlererrors.NewRetryableError(-2)
	tests := []struct {
		name   string
		errors []error
		calls  int
		want   error
	}{
		{"eventual success", []error{first, first, nil}, 3, nil},
		{"last error and fixed budget", []error{first, last}, 3, last},
		{"no retries", []error{zero}, 1, zero},
		{"negative retries", []error{negative}, 1, negative},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &sequenceDB{errors: tt.errors}
			got := NewHandler(db).UpdateUserBalance(context.Background(), 7, 100)
			if db.calls != tt.calls {
				t.Fatalf("database calls = %d, want %d", db.calls, tt.calls)
			}
			if got != tt.want {
				t.Fatalf("error = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChangedErrorAfterRetry(t *testing.T) {
	retry := handlererrors.NewRetryableError(2)
	notFound := handlererrors.NewNotFoundError(7)
	wrapped := fmt.Errorf("database: %w", notFound)
	db := &sequenceDB{errors: []error{retry, wrapped}}
	err := NewHandler(db).UpdateUserBalance(context.Background(), 7, 100)
	var additional *handlererrors.AdditionalMessageError
	if db.calls != 2 || !errors.As(err, &additional) || !errors.Is(err, notFound) {
		t.Fatalf("changed error was not handled: calls=%d, error=%v", db.calls, err)
	}
}
