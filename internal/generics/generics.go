package generics

import "strconv"

/*
Page represents a paginated result set with metadata.

Fields:
- Page: Current page number (1-indexed)
- Size: Number of records returned for the current page
- TotalPages: Total number of pages based on TotalResults and Size
- TotalResults: Total number of records found in the database
- Content: Slice containing the actual data records for the current page
*/
type Page[T any] struct {
	Page         int
	Size         int
	TotalPages   int
	TotalResults int
	Content      []T
}

func StringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}
