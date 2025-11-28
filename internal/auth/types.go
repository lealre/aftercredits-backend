package auth

type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email" `
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}
