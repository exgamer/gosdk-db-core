package pagination

// Paginated постраничный список
type Paginated[E interface{}] struct {
	Items      []*E        `json:"items"`
	Pagination *Pagination `json:"pagination"`
}
