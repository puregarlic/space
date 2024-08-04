package types

type Post struct {
	ID         string           `clover:""`
	CreatedAt  int64            `clover:"createdAt"`
	Type       string           `clover:"type"`
	Properties map[string][]any `clover:"properties"`
}
