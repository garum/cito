package service

import (
	"cito/server/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"
)

// OAuth2TokenExchanger interface for OAuth2 token exchange (enables mocking)
type OAuth2TokenExchanger interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
}

type AuthService struct {
	OAuthConfig  OAuth2TokenExchanger
	httpClient   *http.Client
	githubAPIURL string
}

func (as *AuthService) fetchGitHubUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	httpClient := as.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	slog.Info("client eamil body gh:", "email", body)
	type Email struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	var emails []Email

	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("failed to parse GitHub emails: %w", err)

	}
	slog.Info("client emails unmarsh body gh:", "emails", emails)

	for _, email := range emails {
		if email.Primary == true {
			return email.Email, nil
		}
	}
	return "", errors.New("No email found")

}

// fetchGitHubUser fetches user information from GitHub API
func (as *AuthService) fetchGitHubUser(accessToken string) (*model.GitHubUser, error) {

	apiURL := "https://api.github.com/user"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	httpClient := as.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	slog.Info("client body gh:", "body", body)
	var githubUser model.GitHubUser
	if err := json.Unmarshal(body, &githubUser); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub user: %w", err)
	}

	if githubUser.Email == "" {
		githubUser.Email, err = as.fetchGitHubUserEmail(accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Email: %w", err)
		}
	}

	return &githubUser, nil
}
