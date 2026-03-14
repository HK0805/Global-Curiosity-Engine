package main

import (
	"net/http"
	"testing"
)

func TestGDELTThrottleMessage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "http 429",
			statusCode: http.StatusTooManyRequests,
			body:       "Please limit requests to one every 5 seconds.",
			want:       true,
		},
		{
			name:       "plain text too many requests",
			statusCode: http.StatusOK,
			body:       "Too Many Requests",
			want:       true,
		},
		{
			name:       "rate limit text",
			statusCode: http.StatusOK,
			body:       "Please limit requests to one every 5 seconds.",
			want:       true,
		},
		{
			name:       "json response",
			statusCode: http.StatusOK,
			body:       `{"articles":[]}`,
			want:       false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, got := gdeltThrottleMessage(testCase.statusCode, testCase.body)
			if got != testCase.want {
				t.Fatalf("gdeltThrottleMessage(%d, %q) = %t, want %t", testCase.statusCode, testCase.body, got, testCase.want)
			}
		})
	}
}
