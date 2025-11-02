package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lealre/movies-backend/internal/api"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
)

func NewServer(db *mongo.Client) http.Handler {
	mux := http.NewServeMux()

	a := api.NewAPI(mongodb.NewDB(db))

	mux.HandleFunc("GET /titles", a.GetTitles)
	mux.HandleFunc("GET /titles/{id}/ratings", a.GetTitleRatings)
	mux.HandleFunc("POST /titles", a.AddTitle)
	mux.HandleFunc("PATCH /titles/{id}", a.SetWatched)
	mux.HandleFunc("DELETE /titles/{id}", a.DeleteTitle)

	mux.HandleFunc("GET /ratings/{id}", a.GetRatingById)
	mux.HandleFunc("POST /ratings/batch", a.GetRatingsBatchByTitleIDs)
	mux.HandleFunc("POST /ratings", a.AddRating)
	mux.HandleFunc("PATCH /ratings/{id}", a.UpdateRating)

	mux.HandleFunc("GET /comments/{titleId}", a.GetCommentsByTitleID)
	mux.HandleFunc("PATCH /comments/{id}", a.UpdateComment)
	mux.HandleFunc("POST /comments", a.AddComment)
	mux.HandleFunc("DELETE /comments/{id}", a.DeleteComment)

	mux.HandleFunc("GET /users", a.GetUsers)

	return RequestIDMiddleware(mux)
}

func ListenAndServe(db *mongo.Client) error {
	server := &http.Server{
		Addr:    ":8080",
		Handler: NewServer(db),
	}
	log.Println("Server running on :8080")
	err := server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("error while starting server: %v", err)
	}
	log.Println("Server started listening on port 8080")
	return nil
}
