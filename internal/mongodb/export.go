package mongodb

import "github.com/lealre/movies-backend/internal/models"

// Exported Db -> model wrappers for the one-time Mongo -> Postgres data
// migration (internal/pgmigration), which decodes mongodump BSON into this
// package's Db structs and needs the ground-truth mapping to the neutral
// models. Pure delegation so the mapping logic stays defined in one place.

func UserDbToModel(u UserDb) models.User { return userDbToModel(u) }

func TitleDbToModel(t TitleDb) models.Title { return titleDbToModel(t) }

func UserRatingDbToModel(r RatingDb) models.UserRating { return userRatingDbToModel(r) }

func CommentDbToModel(c CommentDb) models.Comment { return commentDbToModel(c) }

func GroupDbToModel(g GroupDb) models.Group { return groupDbToModel(g) }
