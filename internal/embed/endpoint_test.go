package embed

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndpointEmbed(t *testing.T) {
	var gotReq embeddingRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotReq)
		// Respond out of order to prove index-based reordering.
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float32{0, 2}},
				{"index": 0, "embedding": []float32{3, 0}},
			},
		})
	}))
	defer srv.Close()

	e := NewEndpoint(srv.URL+"/v1", "test-model")
	vecs, err := e.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if gotReq.Model != "test-model" || len(gotReq.Input) != 2 {
		t.Errorf("bad request sent: %+v", gotReq)
	}
	// Reordered by index and L2-normalized.
	if vecs[0][0] != 1 || vecs[0][1] != 0 {
		t.Errorf("vector 0 = %v, want [1 0]", vecs[0])
	}
	if vecs[1][0] != 0 || vecs[1][1] != 1 {
		t.Errorf("vector 1 = %v, want [0 1]", vecs[1])
	}
}

func TestEndpointErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := NewEndpoint(srv.URL, "missing").Embed(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error from 404 endpoint")
	}
}

func TestNormalize(t *testing.T) {
	v := []float32{3, 4}
	normalize(v)
	if math.Abs(float64(v[0])-0.6) > 1e-6 || math.Abs(float64(v[1])-0.8) > 1e-6 {
		t.Errorf("normalize = %v, want [0.6 0.8]", v)
	}

	zero := []float32{0, 0}
	normalize(zero) // must not NaN
	if zero[0] != 0 || zero[1] != 0 {
		t.Errorf("zero vector changed: %v", zero)
	}
}
