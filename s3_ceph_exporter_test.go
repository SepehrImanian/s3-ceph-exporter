package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateSignature(t *testing.T) {
	signature := generateSignature("GET", "/admin/user", "your_secret_key_here")
	expectedSignature := "your_expected_signature_here"
	if signature != expectedSignature {
		t.Errorf("Expected signature: %s, but got: %s", expectedSignature, signature)
	}
}

func TestCreateRequest(t *testing.T) {
	// Create a mock server for testing
	mockServer := createMockServer()
	defer mockServer.Close()

	req, err := createRequest(mockServer.URL, "your_access_key_here", "your_signature_here")
	if err != nil {
		t.Errorf("Error creating request: %v", err)
	}

	// Perform assertions on the request object
	if req.Method != "GET" {
		t.Errorf("Expected request method 'GET', but got '%s'", req.Method)
	}
	if req.URL.String() != mockServer.URL {
		t.Errorf("Expected request URL '%s', but got '%s'", mockServer.URL, req.URL.String())
	}
	// Add more assertions for headers, etc.
}

func TestUpdateBucketStatsMap(t *testing.T) {
	collector := &S3Collector{}
	testBuckets := []BucketStats{
		{Name: "bucket1"},
		{Name: "bucket2"},
	}
	collector.updateBucketStatsMap(testBuckets)

	// Verify that the bucketStats map is updated correctly
	if len(collector.bucketStats) != len(testBuckets) {
		t.Errorf("Expected %d buckets in map, but got %d", len(testBuckets), len(collector.bucketStats))
	}
	// Add more assertions as needed
}

func createMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock response here
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"bucket":"test_bucket"}`))
	}))
}
