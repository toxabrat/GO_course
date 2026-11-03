package handler

import (
	"context"
	"errors"

	handlererrors "github.com/tcarzverey/course-go-python/homeworks/hw1/handler/errors"
)

type Handler struct {
	db UsersDB
}

func NewHandler(db UsersDB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) UpdateUserBalance(ctx context.Context, userID, balance int64) error {
	retryLimit := 0
	for attempt := 0; ; attempt++ {
		err := h.db.UpdateBalance(ctx, userID, balance)
		if err == nil {
			return nil
		}
		var retryable *handlererrors.RetryableError
		var notFound *handlererrors.NotFoundError
		var additional *handlererrors.AdditionalMessageError
		switch {
		case errors.As(err, &retryable):
			if attempt == 0 {
				retryLimit = retryable.RetryCount()
			}
			if attempt >= retryLimit {
				return err
			}
		case errors.As(err, &notFound):
			return handlererrors.NewAdditionalMessageError(err, "update balance for user %d", userID)
		case errors.As(err, &additional):
			return err
		default:
			panic(err)
		}
	}
}
