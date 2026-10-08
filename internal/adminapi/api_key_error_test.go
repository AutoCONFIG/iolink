package adminapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyError_infrastructureFailureReturns500(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	apiKeyError(ctx, errors.New("database unavailable"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("infrastructure status=%d", recorder.Code)
	}
	if recorder.Body.String() != `{"error":"internal_error"}` {
		t.Fatal("internal error details exposed")
	}
}
