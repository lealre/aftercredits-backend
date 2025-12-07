package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/mongodb"
	"github.com/lealre/movies-backend/internal/services/comments"
)

func (api *API) GetCommentsByTitleID(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title Id is required")
		return
	}

	commentsList, err := comments.GetCommentsByTitleId(api.Db, r.Context(), titleId, currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusOK, "Error while seraching comments in Database")
		return
	}

	respondWithJSON(w, http.StatusOK, comments.AllCommentsFromTitle{Comments: commentsList})
}

func (api *API) UpdateComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

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

	if err := comments.UpdateComment(api.Db, r.Context(), commentId, currentUser.Id, updateReq); err != nil {
		if err == mongodb.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Comment with id %s not found", commentId))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update comment")
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: "Comment updated successfully"})

}

func (api *API) AddComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	var newComment comments.NewComment
	if err := json.NewDecoder(r.Body).Decode(&newComment); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	if ok, err := api.Db.GroupContainsTitle(r.Context(), newComment.GroupId, newComment.TitleId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	createdComment, err := comments.AddComment(api.Db, r.Context(), newComment, currentUser.Id)
	if err != nil {
		if statusCode, ok := comments.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add comment")
		return
	}

	respondWithJSON(w, http.StatusCreated, createdComment)
}

func (api *API) DeleteComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	commentId := r.PathValue("id")
	if commentId == "" {
		respondWithError(w, http.StatusBadRequest, "Comment id is required")
		return
	}

	// TODO: Add authorization

	if deletedCount, err := comments.DeleteComment(api.Db, r.Context(), commentId, currentUser.Id); err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error while deleting comment")
		return
	} else if deletedCount == 0 {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Comment with id %s not found", commentId))
		return
	}

	respondWithJSON(w, http.StatusOK, DefaultResponse{Message: fmt.Sprintf("Comment with id %s deleted successfully", commentId)})
}
