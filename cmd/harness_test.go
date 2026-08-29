package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// runCommand executes the real command tree against an httptest server,
// the way a person would from a shell, and returns what was printed. Every
// flag is reset to its default afterwards so tests do not leak state.
func runCommand(t *testing.T, server *httptest.Server, args ...string) (string, error) {
	t.Helper()
	t.Setenv("TAEL_CONFIG", filepath.Join(t.TempDir(), "tael.yaml"))
	t.Setenv(envAPIToken, "")
	t.Setenv(envBaseURL, "")
	t.Setenv(envWorkspace, "")
	resetFlags(rootCmd)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs(append([]string{"--token", "test-token", "--base-url", server.URL}, args...))
	t.Cleanup(func() {
		resetFlags(rootCmd)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	_, executeError := rootCmd.ExecuteContextC(context.Background())
	return out.String(), executeError
}

func resetFlags(command *cobra.Command) {
	reset := func(flag *pflag.Flag) {
		if slice, isSlice := flag.Value.(pflag.SliceValue); isSlice {
			_ = slice.Replace([]string{})
		} else {
			_ = flag.Value.Set(flag.DefValue)
		}
		flag.Changed = false
	}
	command.Flags().VisitAll(reset)
	command.PersistentFlags().VisitAll(reset)
	for _, child := range command.Commands() {
		resetFlags(child)
	}
}

// route is one canned answer: method and path in, status and body out.
type route struct {
	method string
	path   string
	status int
	body   string
}

// newAPIServer serves the routes given and answers 404 with a detail for
// anything else. Requests are recorded so a test can check what was sent.
func newAPIServer(t *testing.T, routes ...route) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	recorded := &[]recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*recorded = append(*recorded, recordedRequest{Method: request.Method, Path: request.URL.RequestURI(), Body: string(body)})
		writer.Header().Set("Content-Type", "application/json")
		for _, candidate := range routes {
			if candidate.method == request.Method && candidate.path == request.URL.Path {
				writer.WriteHeader(candidate.status)
				_, _ = writer.Write([]byte(candidate.body))
				return
			}
		}
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"detail":"Not found"}`))
	}))
	t.Cleanup(server.Close)
	return server, recorded
}

type recordedRequest struct {
	Method string
	Path   string
	Body   string
}

// lastRequest is the most recent request with this method and path (the
// query string aside).
func lastRequest(recorded *[]recordedRequest, method string, path string) *recordedRequest {
	for index := len(*recorded) - 1; index >= 0; index-- {
		request := (*recorded)[index]
		requestPath, _, _ := strings.Cut(request.Path, "?")
		if request.Method == method && requestPath == path {
			return &request
		}
	}
	return nil
}

func decodeBody(t *testing.T, request *recordedRequest) map[string]any {
	t.Helper()
	if request == nil {
		t.Fatalf("no request recorded")
	}
	var decoded map[string]any
	if unmarshalError := json.Unmarshal([]byte(request.Body), &decoded); unmarshalError != nil {
		t.Fatalf("request body %q is not JSON: %v", request.Body, unmarshalError)
	}
	return decoded
}

func mustContain(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("output is missing %q\ngot:\n%s", want, output)
		}
	}
}

// platformWords are what the product never says; every rendered text is
// checked against them.
var platformWords = []string{"ankra", "stack", "profile", "helm", "cluster", "operation"}

func mustSpeakTael(t *testing.T, output string) {
	t.Helper()
	lower := strings.ToLower(output)
	for _, word := range platformWords {
		if strings.Contains(lower, word) {
			t.Errorf("output says %q:\n%s", word, output)
		}
	}
}
