package model

import "context"

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

type userContextKey struct{}

func NewContextWithUserValue(ctx context.Context, user *UserModel) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func GetUserValueFromContext(ctx context.Context) (*UserModel, bool) {
	value, ok := ctx.Value(userContextKey{}).(*UserModel)
	return value, ok
}
