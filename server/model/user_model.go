package model

type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

type UserModel struct {
	ID           int
	GithubID     int64
	Username     string
	Email        string
	AccessToken  string
	SessionToken string
}
