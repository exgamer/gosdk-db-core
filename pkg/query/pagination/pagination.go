package pagination

type Pagination struct {
	TotalRecords int64 `json:"total"`
	TotalPage    int   `json:"last_page"`
	Offset       int   `json:"offset"`
	From         int   `json:"from"`
	To           int   `json:"to"`
	Limit        int   `json:"per_page"`
	Page         int   `json:"current_page"`
	PrevPage     int   `json:"prev_page"`
	NextPage     int   `json:"next_page"`
}
