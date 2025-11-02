package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/lealre/movies-backend/internal/services/users"
	"go.mongodb.org/mongo-driver/mongo"
)

func (api *API) GetCommentsByTitleID(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title Id is required")
		return
	}

	ctx := context.Background()
	if ok, err := api.Db.TitleExists(ctx, titleId); !ok {
		respondWithError(w, http.StatusBadRequest, "Title Id not found")
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error while searching Title in database")
		return
	}

	commentsList, err := comments.GetCommentsByTitleId(ctx, titleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusOK, "Error while seraching comments in Database")
	}

	if commentsList == nil {
		commentsList = []comments.Comment{}
	}

	respondWithJSON(w, http.StatusOK, comments.AllCommentsFromTitle{Comments: commentsList})
}

func (api *API) UpdateComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	commentId := r.PathValue("id")
	if commentId == "" {
		respondWithError(w, http.StatusBadRequest, "Comment id is required")
		return
	}

	var updateReq comments.UpdateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	ctx := context.Background()
	if err := comments.UpdateComment(ctx, commentId, updateReq); err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Comment with id %s not found", commentId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update comment")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Comment updated successfully"})

}

func (api *API) AddComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	var comment comments.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	ctx := context.Background()
	if ok, err := users.CheckIfUserExist(ctx, comment.UserId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("User with id %s not found", comment.UserId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking user")
		return
	}

	if ok, err := api.Db.TitleExists(ctx, comment.TitleId); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Title with id %s not found", comment.TitleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while checking title")
		return
	}

	createdComment, err := comments.AddComment(ctx, comment)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			respondWithError(w, http.StatusBadRequest, "Comment already exists for this user and title")
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add comment")
		return
	}

	respondWithJSON(w, http.StatusCreated, createdComment)
}

func (a *API) DeleteComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())

	commentId := r.PathValue("id")
	if commentId == "" {
		respondWithError(w, http.StatusBadRequest, "Comment id is required")
		return
	}

	ctx := context.Background()
	if deletedCount, err := comments.DeleteComment(ctx, commentId); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting comment")
		return
	} else if deletedCount == 0 {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Comment with id %s not found", commentId))
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("Comment with id %s deleted successfully", commentId)})
}
