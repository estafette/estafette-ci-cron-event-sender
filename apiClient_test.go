package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetToken(t *testing.T) {
	t.Run("ReturnsToken", func(t *testing.T) {

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		ctx := context.Background()
		apiBaseURL := os.Getenv("API_BASE_URL")
		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")
		apiClient := NewApiClient(apiBaseURL)

		// act
		token, err := apiClient.GetToken(ctx, clientID, clientSecret)

		assert.Nil(t, err)
		assert.True(t, len(token) > 0)
	})
}
