package pagination

type Paging struct {
	Page     uint     `json:"page"`
	OrderBy  []string `json:"order_by"`
	Limit    uint     `json:"limit"`
	MaxLimit uint
	ShowSQL  bool
}
