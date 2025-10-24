package generics

type Page[T any] struct {
	Page         int
	TotalPages   int
	TotalResults int
	Content      []T
}
