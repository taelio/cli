package cmd

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"tael.io/cli/internal/client"
)

// newStreamServer serves whatever answer decides, so a test can hand back
// server-sent events for one path and JSON for another.
func newStreamServer(t *testing.T, answer func(request *http.Request) (int, string)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		status, body := answer(request)
		if strings.HasPrefix(body, "event:") || strings.HasPrefix(body, "id:") || strings.HasPrefix(body, "data:") {
			writer.Header().Set("Content-Type", "text/event-stream")
		} else {
			writer.Header().Set("Content-Type", "application/json")
		}
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

const answerStream = "event: tool_start\ndata: {\"tool\":\"k8s_get_pods\"}\n\n" +
	"event: tool_result\ndata: {\"tool\":\"k8s_get_pods\",\"ok\":true}\n\n" +
	"event: content_delta\ndata: {\"delta\":{\"text\":\"web restarts because \"}}\n\n" +
	"event: content_delta\ndata: {\"text\":\"DATABASE_URL is unset.\"}\n\n" +
	"event: tool_start\ndata: {\"name\":\"get_ankra_stack_status\"}\n\n" +
	"event: tool_result\ndata: {\"name\":\"get_ankra_stack_status\",\"status\":\"error\"}\n\n" +
	"event: action_proposal\ndata: {\"action_id\":\"act_1\",\"title\":\"Add DATABASE_URL\",\"summary\":\"Set it from the managed database.\",\"risk\":\"Low\"}\n\n" +
	"event: ask_question\ndata: {\"question\":\"Shall I?\"}\n\n" +
	"event: turn_completed\ndata: {}\n\n" +
	"event: content_delta\ndata: {\"text\":\"never printed\"}\n\n"

func TestAskStreamsTheAnswerInTaelsWords(t *testing.T) {
	var body string
	server := newStreamServer(t, func(request *http.Request) (int, string) {
		if request.URL.Path == "/api/v1/apps" {
			return http.StatusOK, `{"apps":[{"id":"app_1","name":"web"}]}`
		}
		raw := make([]byte, request.ContentLength)
		_, _ = request.Body.Read(raw)
		body = string(raw)
		return http.StatusOK, answerStream
	})
	output, runError := runCommand(t, server, "ask", "why does web restart?", "--app", "web")
	if runError != nil {
		t.Fatalf("tael ask: %v", runError)
	}
	expected := "  · Looked at the pods\n" +
		"web restarts because DATABASE_URL is unset.\n" +
		"  · Looked at the infrastructure status\n" +
		"  · Looked at the infrastructure status — that did not answer\n" +
		"Tael proposes: Add DATABASE_URL — Set it from the managed database. (low risk)\n" +
		"Decide it with `tael needs-you` once it is on the record.\n" +
		"Tael asks: Shall I?\n"
	if output != expected {
		t.Fatalf("tael ask output mismatch\ngot:\n%s\nwant:\n%s", output, expected)
	}
	mustSpeakTael(t, output)
	if !strings.Contains(body, `"message":"why does web restart?"`) || !strings.Contains(body, `"appId":"app_1"`) || !strings.Contains(body, `"surface":"tael-cli"`) {
		t.Fatalf("ask body = %s", body)
	}

	jsonOutput, jsonError := runCommand(t, server, "ask", "why?", "-o", "json")
	if jsonError != nil || !strings.Contains(jsonOutput, `{"kind":"tool_start","label":"Looked at the pods"}`) || !strings.Contains(jsonOutput, `{"kind":"end"}`) {
		t.Fatalf("tael ask -o json = %q, %v", jsonOutput, jsonError)
	}
}

func TestAskRefusals(t *testing.T) {
	notReady := newStreamServer(t, func(*http.Request) (int, string) {
		return http.StatusServiceUnavailable, `{"detail":"Tael AI is still being set up for this workspace. Try again in a minute."}`
	})
	_, refusal := runCommand(t, notReady, "ask", "hello")
	if refusal == nil || !strings.Contains(refusal.Error(), "still being set up") {
		t.Fatalf("tael ask while not ready = %v", refusal)
	}
	_, blank := runCommand(t, notReady, "ask", "  ")
	if blank == nil || exitCodeFor(blank) != exitUsage {
		t.Fatalf("tael ask with nothing = %v, want a usage error", blank)
	}
	failing := newStreamServer(t, func(*http.Request) (int, string) {
		return http.StatusOK, "event: content_delta\ndata: {\"text\":\"Let me\"}\n\nevent: error\ndata: {\"detail\":\"The model timed out.\"}\n\n"
	})
	output, streamError := runCommand(t, failing, "ask", "hello")
	if streamError == nil || !strings.Contains(streamError.Error(), "The model timed out.") || !strings.HasPrefix(output, "Let me\n") {
		t.Fatalf("tael ask with an error frame = %q, %v", output, streamError)
	}
}

func TestToolLabelNeverLeaksAnIdentifier(t *testing.T) {
	identifier := regexp.MustCompile(`^[a-z0-9]+(?:[_-][a-z0-9]+)+$`)
	cases := map[string]string{
		"get_pods":               "Looked at the pods",
		"k8s_get_pod_logs":       "Read the logs",
		"ankra_get_cluster":      "Looked at your infrastructure",
		"get_stack":              "Looked at what is installed",
		"describe_ingress_rules": "Looked at the ingress rules",
		"get_ankra_thing":        "Looked at the infrastructure thing",
		"frobnicate":             "Checked something",
		"":                       "Checked something",
		"tail_logs_v2":           "Read the logs",
	}
	for name, want := range cases {
		if got := toolLabel(name); got != want {
			t.Errorf("toolLabel(%q) = %q, want %q", name, got, want)
		}
	}
	for name, label := range toolLabels {
		if identifier.MatchString(label) {
			t.Errorf("toolLabels[%s] = %q is an identifier", name, label)
		}
		mustSpeakTael(t, label)
	}
	if text := textFromFrame(client.SSEFrame{Event: "message", Data: "plain words"}); text != "plain words" {
		t.Fatalf("textFromFrame(bare text) = %q", text)
	}
	if frame := classifyChatFrame(client.SSEFrame{Event: "something_new", Data: "{}"}); frame.Kind != "ignore" {
		t.Fatalf("an unknown frame = %+v, want ignored", frame)
	}
}
