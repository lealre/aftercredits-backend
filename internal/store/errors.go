// Package store defines the storage-neutral persistence interface and the
// sentinel errors its implementations return, so services depend on the
// contract rather than on any concrete database.
package store

import "errors"

var (
	ErrRecordNotFound   = errors.New("record not found in the database")
	ErrDuplicatedRecord = errors.New("duplicated record not allowed")
)
