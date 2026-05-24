package benchsvc

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestHandleCompareModels_NormalizesCSVQuery(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		matrix: &bench.ModelMatrix{
			Models:    []string{"sonnet", "opus"},
			Scenarios: []string{"s1", "s2"},
			Cells:     map[string]map[string]bench.ModelMatrixCell{},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "tenant-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/bench/compare/models?models=sonnet,%20opus,sonnet,,&scenarios=s1,%20s2,s1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if want := []string{"sonnet", "opus"}; !reflect.DeepEqual(repo.lastModels, want) {
		t.Fatalf("models = %#v, want %#v", repo.lastModels, want)
	}
	if want := []string{"s1", "s2"}; !reflect.DeepEqual(repo.lastScenarios, want) {
		t.Fatalf("scenarios = %#v, want %#v", repo.lastScenarios, want)
	}
}
