package transport

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Page struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (page Page) Normalize() Page {
	if page.Page == 0 {
		page.Page = 1
	}
	if page.PageSize == 0 {
		page.PageSize = DefaultPageSize
	}
	return page
}

func (page Page) Valid() bool {
	return page.Page > 0 && page.PageSize > 0 && page.PageSize <= MaxPageSize
}

func (page Page) Offset() int {
	return (page.Page - 1) * page.PageSize
}

type PageData[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
