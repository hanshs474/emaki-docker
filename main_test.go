package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type reply struct {
	status int
	body   string
}

// serve replays recorded response shapes in order and records each request.
func serve(t *testing.T, replies []reply) *[]map[string]any {
	t.Helper()
	var seen []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		b["_path"] = r.URL.Path
		seen = append(seen, b)
		rp := replies[0]
		replies = replies[1:]
		w.WriteHeader(rp.status)
		_, _ = w.Write([]byte(rp.body))
	}))
	t.Cleanup(srv.Close)
	baseURL = srv.URL
	sleep = func(time.Duration) {}
	return &seen
}

func TestVideo(t *testing.T) {
	seen := serve(t, []reply{
		{200, `{"success":true,"data":{"id":"v1","status":"generating"}}`},
		{200, `{"success":true,"data":{"id":"v1","status":"generating"}}`},
		{200, `{"success":true,"data":{"id":"v1","status":"completed","videoUrl":"https://cdn.emaki.ai/v1.mp4"}}`},
	})
	var out, errb bytes.Buffer
	if code := run([]string{"-duration", "6", "a fox"}, "sk-x", &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if strings.TrimSpace(out.String()) != "https://cdn.emaki.ai/v1.mp4" {
		t.Fatal(out.String())
	}
	s := *seen
	if s[0]["_path"] != "/api/video/ai" || s[0]["model"] != "seedance_2_5" || s[0]["parameters"].(map[string]any)["duration"] != 6.0 {
		t.Fatalf("submit body %v", s[0])
	}
	if s[1]["_path"] != "/api/video/status" || s[1]["videoId"] != "v1" {
		t.Fatalf("status body %v", s[1])
	}
}

func TestImageDefaultModel(t *testing.T) {
	seen := serve(t, []reply{
		{200, `{"success":true,"data":{"id":"i1","status":"generating"}}`},
		{200, `{"success":true,"data":{"id":"i1","status":"completed","videoUrl":"https://cdn.emaki.ai/i1.png"}}`},
	})
	var out, errb bytes.Buffer
	if run([]string{"-image", "a mug"}, "sk-x", &out, &errb) != 0 || (*seen)[0]["model"] != "nano_banana_2_lite" {
		t.Fatal(errb.String())
	}
	if _, has := (*seen)[0]["parameters"]; has {
		t.Fatal("empty parameters should be omitted")
	}
}

func TestFailedCarriesCode(t *testing.T) {
	serve(t, []reply{
		{200, `{"success":true,"data":{"id":"f1","status":"generating"}}`},
		{200, `{"success":true,"data":{"id":"f1","status":"failed","failureCode":"provider_error"}}`},
	})
	var out, errb bytes.Buffer
	if run([]string{"x"}, "sk-x", &out, &errb) != 1 || !strings.Contains(errb.String(), "provider_error") {
		t.Fatal(errb.String())
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		kind   string
		exit   int
	}{
		{401, `{"error":"Invalid API key. Create one at https://emaki.ai/settings/apikeys"}`, "auth", 3},
		{402, `{"error":"Insufficient credits"}`, "quota", 4},
		{403, `{"error":"This model requires a purchase"}`, "plan", 4},
		{409, `{"error":"Video generation is already in progress"}`, "busy", 1},
		{429, `{"error":"cooling down","code":"content_cooldown"}`, "rejected", 1},
		{400, `{"error":"blocked","code":"content_blocked"}`, "rejected", 1},
		{400, `{"error":"Missing required parameters: prompt and model"}`, "validation", 1},
		{500, `{"error":"Internal error"}`, "other", 1},
	} {
		serve(t, []reply{{c.status, c.body}})
		var out, errb bytes.Buffer
		code := run([]string{"x"}, "sk-x", &out, &errb)
		if code != c.exit || !strings.HasPrefix(errb.String(), c.kind+":") {
			t.Fatalf("%d: exit %d %q", c.status, code, errb.String())
		}
	}
}

func TestCredits(t *testing.T) {
	serve(t, []reply{{200, `{"code":0,"message":"ok","data":{"remainingCredits":42,"hasPurchasedCredits":false}}`}})
	var out, errb bytes.Buffer
	if run([]string{"-credits"}, "sk-x", &out, &errb) != 0 || strings.TrimSpace(out.String()) != "42" {
		t.Fatal(out.String(), errb.String())
	}
	serve(t, []reply{{200, `{"code":-1,"message":"Invalid API key. Create one at https://emaki.ai/settings/apikeys"}`}})
	out.Reset()
	if run([]string{"-credits"}, "sk-x", &out, &errb) != 3 {
		t.Fatal("code -1 should be auth")
	}
}
