package pagination

type Pagination struct {
	TotalRecords uint64 `json:"total"`
	TotalPage    uint   `json:"last_page"`
	Offset       uint   `json:"offset"`
	From         uint   `json:"from"`
	To           uint   `json:"to"`
	Limit        uint   `json:"per_page"`
	Page         uint   `json:"current_page"`
	PrevPage     uint   `json:"prev_page"`
	NextPage     uint   `json:"next_page"`
}
