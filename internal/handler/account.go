package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/marquisccel/banking-peak-load-prototype/internal/service"
)

type AccountHandler struct {
	svc service.AccountService
}

func NewAccountHandler(svc service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) GetBalance(c *echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid account id"})
	}

	acc, err := h.svc.GetBalance(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "account not found"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":         acc.ID,
		"name":       acc.Name,
		"balance":    acc.Balance,
		"updated_at": acc.UpdatedAt,
	})
}
