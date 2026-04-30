package comments

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

type Comment struct {
	AuthorName string
	Body       string
	Status     string
}

func NewComment(authorName, body string) Comment {
	return Comment{
		AuthorName: authorName,
		Body:       body,
		Status:     StatusPending,
	}
}
