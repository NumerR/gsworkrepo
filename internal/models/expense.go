package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Expense struct {
	ID          int64
	Amount      int64 // в копейках
	Description string
	Date        string // YYYY-MM-DD
	CategoryID  int64  // 0 — категория не выбрана
	CategoryName string // приходит из JOIN с categories
}

// CategoryText — название категории для отображения.
func (e Expense) CategoryText() string {
	if e.CategoryName == "" {
		return "Без категории"
	}
	return e.CategoryName
}

// AmountInput — сумма для поля формы: "1234.50".
func (e Expense) AmountInput() string {
	if e.Amount == 0 {
		return ""
	}
	return fmt.Sprintf("%d.%02d", e.Amount/100, e.Amount%100)
}

// AmountText — сумма для отображения: "1 234,50 ₽".
func (e Expense) AmountText() string {
	rub := strconv.FormatInt(e.Amount/100, 10)

	var b strings.Builder
	for i, digit := range rub {
		if i > 0 && (len(rub)-i)%3 == 0 {
			b.WriteRune('\u00a0')
		}
		b.WriteRune(digit)
	}

	return fmt.Sprintf("%s,%02d\u00a0₽", b.String(), e.Amount%100)
}

// DateText — дата для отображения: "24.10.2026".
func (e Expense) DateText() string {
	t, err := time.Parse(time.DateOnly, e.Date)
	if err != nil {
		return e.Date
	}
	return t.Format("02.01.2006")
}
