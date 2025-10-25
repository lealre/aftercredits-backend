package mongodb

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrRecordNotFound = errors.New("record not found in the database")

func ResolveFilterAndOptionsSearch(args ...any) (bson.M, []*options.FindOptions) {
	filter := bson.M{}
	var opts []*options.FindOptions

	for _, arg := range args {
		switch v := arg.(type) {
		case bson.M:
			filter = v
		case *options.FindOptions:
			opts = append(opts, v)
		default:
			// Just ignore if no args match
		}
	}

	return filter, opts
}
