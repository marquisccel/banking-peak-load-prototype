package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/marquisccel/banking-peak-load-prototype/internal/domain/transaction"
	"github.com/marquisccel/banking-peak-load-prototype/internal/handler/request"
	"github.com/marquisccel/banking-peak-load-prototype/internal/service"
)

type TransactionHandler struct {
	svc service.TransactionService
}

func NewTransactionHandler(svc service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) CreateTransaction(c *echo.Context) error {
	var req request.CreateTransaction
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if req.SourceAccount == 0 || req.DestAccount == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "source_account and dest_account are required"})
	}
	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
	}

	tx, err := h.svc.CreateTransaction(c.Request().Context(), service.CreateTransactionInput{
		SourceAccount: req.SourceAccount,
		DestAccount:   req.DestAccount,
		Amount:        req.Amount,
	})
	if err != nil {
		if errors.Is(err, service.ErrInsufficientFunds) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "insufficient funds")
		}
		if errors.Is(err, service.ErrAccountNotFound) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "account not found")
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save transaction"})
	}

	statusCode := http.StatusCreated // 201 for sync (completed)
	if tx.Status == transaction.StatusPending {
		statusCode = http.StatusAccepted // 202 for async (pending)
	}

	return c.JSON(statusCode, map[string]any{
		"id":             tx.ID,
		"source_account": tx.SourceAccount,
		"dest_account":   tx.DestAccount,
		"amount":         tx.Amount,
		"status":         tx.Status,
		"created_at":     tx.CreatedAt,
	})
}

func (h *TransactionHandler) GetTransactionStatus(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	tx, err := h.svc.GetTransactionStatus(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "transaction not found"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":             tx.ID,
		"source_account": tx.SourceAccount,
		"dest_account":   tx.DestAccount,
		"amount":         tx.Amount,
		"status":         tx.Status,
		"created_at":     tx.CreatedAt,
	})
}
