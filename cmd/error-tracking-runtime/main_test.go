package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	"github.com/movebigrocks/extension-sdk/runtimeproto"
	"github.com/stretchr/testify/require"
)

type scopedCall func(context.Context, func(context.Context) error) error

func (f scopedCall) WithAdminContext(ctx context.Context, run func(context.Context) error) error {
	return f(ctx, run)
}

func TestIngestAndInternalRoutesOwnTheirCommitBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/api/envelope", "/api/:projectNumber/envelope", "/1/envelope", runtimeproto.InternalConsumerPathPrefix + "*target", runtimeproto.InternalJobPathPrefix + "*target", "/extensions/error-tracking/issues"} {
		t.Run(path, func(t *testing.T) {
			scopes := 0
			router := gin.New()
			router.Use(errorTrackingRequestScope(scopedCall(func(ctx context.Context, run func(context.Context) error) error { scopes++; return run(ctx) })))
			router.POST(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			requestPath := path
			if path == "/api/:projectNumber/envelope" {
				requestPath = "/api/42/envelope"
			}
			if path == runtimeproto.InternalConsumerPathPrefix+"*target" {
				requestPath = runtimeproto.InternalConsumerPath("test")
			}
			if path == runtimeproto.InternalJobPathPrefix+"*target" {
				requestPath = runtimeproto.InternalJobPath("test")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, requestPath, nil))
			require.Equal(t, http.StatusNoContent, response.Code)
			if path == "/extensions/error-tracking/issues" {
				require.Equal(t, 1, scopes)
			} else {
				require.Zero(t, scopes)
			}
		})
	}
}

type responseSpy struct {
	*httptest.ResponseRecorder
	wrote bool
}

func (s *responseSpy) WriteHeader(status int) { s.wrote = true; s.ResponseRecorder.WriteHeader(status) }
func (s *responseSpy) Write(data []byte) (int, error) {
	s.wrote = true
	return s.ResponseRecorder.Write(data)
}

func TestInternalConsumerCommitFailureCannotAcknowledgeSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := &responseSpy{ResponseRecorder: httptest.NewRecorder()}
	scoper := scopedCall(func(ctx context.Context, run func(context.Context) error) error {
		require.NoError(t, run(ctx))
		require.False(t, response.wrote, "response must wait for the commit outcome")
		return errors.New("commit refused")
	})
	router := gin.New()
	router.Use(errorTrackingRequestScope(scoper))
	runtimehttp.RegisterInternalRoutes(router, map[string]func(context.Context, []byte) error{
		"test": scopedConsumer(scoper, func(context.Context, []byte) error { return nil }),
	}, nil)
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, runtimeproto.InternalConsumerPath("test"), nil))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), "commit refused")
}
