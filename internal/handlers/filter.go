package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gdwork/internal/repository"
)

// expenseFilterView — значения фильтров, которые шаблоны подставляют в поля формы.
type expenseFilterView struct {
	CategoryID int64
	From       string
	To         string
}

func (v expenseFilterView) Filter() repository.ExpenseFilter {
	return repository.ExpenseFilter{CategoryID: v.CategoryID, From: v.From, To: v.To}
}

// Active — задан ли хоть один фильтр; нужен для ссылки сброса и пустого результата.
func (v expenseFilterView) Active() bool {
	return v.CategoryID != 0 || v.From != "" || v.To != ""
}

// Values — те же фильтры обратно в параметры URL. Разобранные на входе значения
// не дают подмешать в ссылку что-то постороннее.
func (v expenseFilterView) Values() url.Values {
	query := url.Values{}
	if v.CategoryID != 0 {
		query.Set("category", strconv.FormatInt(v.CategoryID, 10))
	}
	if v.From != "" {
		query.Set("from", v.From)
	}
	if v.To != "" {
		query.Set("to", v.To)
	}
	return query
}

// Query — фильтры в виде "?category=2&from=..." для action формы и ссылок.
func (v expenseFilterView) Query() string {
	encoded := v.Values().Encode()
	if encoded == "" {
		return ""
	}
	return "?" + encoded
}

// parseExpenseFilter читает необязательные ?category=2&from=2026-10-01&to=2026-10-31.
// Пустое или неразобранное значение означает «фильтр не задан», поэтому битая
// ссылка отдаёт полный список, а не ошибку.
func parseExpenseFilter(r *http.Request) expenseFilterView {
	query := r.URL.Query()

	var view expenseFilterView
	if id, err := strconv.ParseInt(query.Get("category"), 10, 64); err == nil && id > 0 {
		view.CategoryID = id
	}
	view.From = normalizeDate(query.Get("from"))
	view.To = normalizeDate(query.Get("to"))

	return view
}

func normalizeDate(value string) string {
	if _, err := time.Parse(time.DateOnly, value); err != nil {
		return ""
	}
	return value
}
