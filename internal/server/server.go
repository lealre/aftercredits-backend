package server

import (
	"log"
	"net/http"

	"github.com/lealre/movies-backend/internal/api"
)

func ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /titles", api.GetTitles)
	mux.HandleFunc("GET /titles/{id}/ratings", api.GetTitleRatings)
	mux.HandleFunc("POST /titles", api.AddTitle)
	mux.HandleFunc("PATCH /titles/{id}", api.SetWatched)
	mux.HandleFunc("DELETE /titles/{id}", api.DeleteTitle)

	mux.HandleFunc("GET /ratings/{id}", api.GetRatingById)
	mux.HandleFunc("POST /ratings/batch", api.GetRatingsBatchByTitleIDs)
	mux.HandleFunc("POST /ratings", api.AddRating)
	mux.HandleFunc("PATCH /ratings/{id}", api.UpdateRating)

	mux.HandleFunc("GET /comments/{titleId}", api.GetCommentsByTitleID)
	mux.HandleFunc("PATCH /comments/{id}", api.UpdateComment)
	mux.HandleFunc("POST /comments", api.AddComment)
	mux.HandleFunc("DELETE /comments/{id}", api.DeleteComment)

	mux.HandleFunc("GET /users", api.GetUsers)

	wrappedMux := RequestIDMiddleware(mux)
	server := &http.Server{
		Addr:    ":8080",
		Handler: wrappedMux,
	}

	log.Println("Server is running on port 8080")
	return server.ListenAndServe()
}
