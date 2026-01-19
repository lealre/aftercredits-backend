package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lealre/movies-backend/internal/auth"
	"github.com/lealre/movies-backend/internal/logx"
	"github.com/lealre/movies-backend/internal/services/comments"
	"github.com/lealre/movies-backend/internal/services/titles"
)

func (api *API) GetCommentsByTitleIDFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("groupId")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group Id is required")
		return
	}

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title Id is required")
		return
	}

	// This checks if the group exists, if the title is in the group and if the user is in the group
	ok, err := api.Db.GroupContainsTitle(r.Context(), groupId, titleId, currentUser.Id)
	if !ok && err == nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", groupId, titleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	commentsList, err := comments.GetCommentsByTitleId(api.Db, r.Context(), groupId, titleId, currentUser.Id)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, comments.AllCommentsFromTitle{Comments: commentsList})
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

	// Get the title here to be used to know if its a tv or movie
	if ok, err := api.Db.GroupContainsTitle(r.Context(), newComment.GroupId, newComment.TitleId, currentUser.Id); !ok {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", newComment.GroupId, newComment.TitleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	title, err := titles.GetTitleById(api.Db, r.Context(), newComment.TitleId)
	if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	createdComment, err := comments.AddComment(api.Db, r.Context(), newComment, currentUser.Id, title)
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

func (api *API) UpdateComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("groupId")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	commentId := r.PathValue("commentId")
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

	// This checks if the group exists, if the title is in the group and if the user is in the group
	ok, err := api.Db.GroupContainsTitle(r.Context(), groupId, titleId, currentUser.Id)
	if !ok && err == nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", groupId, titleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	updatedComment, err := comments.UpdateComment(api.Db, r.Context(), commentId, currentUser.Id, updateReq)
	if err != nil {
		if statusCode, ok := comments.ErrorMap[err]; ok {
			respondWithError(w, statusCode, formatErrorMessage(err))
			return
		}
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedComment)

}

func (api *API) DeleteComment(w http.ResponseWriter, r *http.Request) {
	logger := logx.FromContext(r.Context())
	currentUser := auth.GetUserFromContext(r.Context())

	groupId := r.PathValue("groupId")
	if groupId == "" {
		respondWithError(w, http.StatusBadRequest, "Group id is required")
		return
	}

	titleId := r.PathValue("titleId")
	if titleId == "" {
		respondWithError(w, http.StatusBadRequest, "Title id is required")
		return
	}

	commentId := r.PathValue("commentId")
	if commentId == "" {
		respondWithError(w, http.StatusBadRequest, "Comment id is required")
		return
	}

	// This checks if the group exists, if the title is in the group and if the user is in the group
	ok, err := api.Db.GroupContainsTitle(r.Context(), groupId, titleId, currentUser.Id)
	if !ok && err == nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Group %s do not have title %s or do not exist.", groupId, titleId))
		return
	} else if err != nil {
		logger.Printf("ERROR: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Unexpected error occurred")
		return
	}

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
