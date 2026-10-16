// Command emaki generates a Seedance 2.5 / Seedance 2.0 clip or an image on
// emaki (https://emaki.ai) and prints the URL of the finished file.
//
//	emaki "a paper lantern drifting down a night river"
//	emaki -model seedance_2 -duration 8 -resolution 720p "a koi pond seen from above"
//	emaki -first-frame https://example.com/statue.jpg "the camera slowly orbits the statue"
//	emaki -image "a ramen stall in the rain, neon reflections"
//	emaki -credits
//
// Set EMAKI_API_KEY (create a key at https://emaki.ai/settings/apikeys).
// Exit codes: 0 ok, 1 failure, 2 usage, 3 auth, 4 out of credits / plan.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	version = "0.1.0"
	baseURL = "https://emaki.ai"
	client  = &http.Client{Timeout: 60 * time.Second}
	sleep   = time.Sleep
)

const keysURL = "https://emaki.ai/settings/apikeys"

// apiError carries the reason a call failed, in terms of what to do next.
type apiError struct {
	Kind    string // auth quota plan busy rejected validation failed timeout other
	Status  int
	Message string
}

func (e *apiError) Error() string { return e.Kind + ": emaki: " + e.Message }

func classify(status int, code string) string {
	switch {
	case status == 401:
		return "auth"
	case status == 402:
		return "quota"
	case status == 403:
		return "plan"
	case status == 409:
		return "busy"
	case code == "content_cooldown" || code == "content_blocked":
		return "rejected"
	case status == 400:
		return "validation"
	}
	return "other"
}

func post(key, path string, body any, out any) error {
	if key == "" {
		return &apiError{Kind: "auth", Message: "an API key is required; create one at " + keysURL}
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "emaki-cli/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return &apiError{Kind: "other", Message: err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct{ Error, Message, Code string }
		_ = json.Unmarshal(raw, &e)
		msg := e.Error
		if msg == "" {
			msg = e.Message
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return &apiError{Kind: classify(resp.StatusCode, e.Code), Status: resp.StatusCode, Message: msg}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &apiError{Kind: "other", Status: resp.StatusCode, Message: "unreadable answer"}
	}
	return nil
}

type record struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	VideoURL    string `json:"videoUrl"`
	FailureCode string `json:"failureCode"`
}

func submit(key, prompt, model string, params map[string]any) (string, error) {
	body := map[string]any{"prompt": prompt, "model": model}
	if len(params) > 0 {
		body["parameters"] = params
	}
	var r struct{ Data record }
	if err := post(key, "/api/video/ai", body, &r); err != nil {
		return "", err
	}
	if r.Data.ID == "" {
		return "", &apiError{Kind: "other", Message: "the service returned no id"}
	}
	return r.Data.ID, nil
}

func status(key, id string) (record, error) {
	var r struct{ Data record }
	err := post(key, "/api/video/status", map[string]string{"videoId": id}, &r)
	return r.Data, err
}

func credits(key string) (int, error) {
	var r struct {
		Code    int
		Message string
		Data    struct{ RemainingCredits int }
	}
	if err := post(key, "/api/user/get-user-credits", map[string]any{}, &r); err != nil {
		return 0, err
	}
	// This endpoint answers refusals with HTTP 200 and code -1.
	if r.Code != 0 {
		kind := "other"
		if strings.Contains(strings.ToLower(r.Message), "api key") {
			kind = "auth"
		}
		return 0, &apiError{Kind: kind, Message: r.Message}
	}
	return r.Data.RemainingCredits, nil
}

func wait(key, id string, every, limit time.Duration) (string, error) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		sleep(every)
		rec, err := status(key, id)
		if err != nil {
			if e, ok := err.(*apiError); ok && e.Kind == "auth" {
				return "", err
			}
			continue // a dropped poll is not a failed run
		}
		switch rec.Status {
		case "completed":
			if rec.VideoURL != "" {
				return rec.VideoURL, nil
			}
		case "failed":
			code := rec.FailureCode
			if code == "" {
				code = "unknown"
			}
			return "", &apiError{Kind: "failed", Message: "the run failed (" + code + ")"}
		}
	}
	return "", &apiError{Kind: "timeout", Message: "deadline passed; the run may still finish in your emaki library (id " + id + ")"}
}

func main() { os.Exit(run(os.Args[1:], os.Getenv("EMAKI_API_KEY"), os.Stdout, os.Stderr)) }

func run(args []string, key string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("emaki", flag.ContinueOnError)
	fs.SetOutput(stderr)
	image := fs.Bool("image", false, "generate an image instead of a clip (default model nano_banana_2_lite)")
	model := fs.String("model", "", "model id: seedance_2_5 (default), seedance_2, seedance_2_fast, seedance_2_mini, minimax_h3, veo_3_1; images nano_banana_2_lite, nano_banana_2, seedream_5_lite, gpt_image_2")
	duration := fs.Int("duration", 0, "clip length in seconds (Seedance 2.5: 4 to 30)")
	resolution := fs.String("resolution", "", "480p, 720p, 1080p (images: 1K, 2K, 4K)")
	aspect := fs.String("aspect", "", "aspect ratio, e.g. 16:9, 9:16, 1:1")
	first := fs.String("first-frame", "", "public https image URL to animate (image to video)")
	paramsJSON := fs.String("params", "", `extra parameters as JSON, e.g. '{"generate_audio":false}'`)
	noWait := fs.Bool("no-wait", false, "print the run id and exit without waiting")
	statusID := fs.String("status", "", "print the status of a run id")
	showCredits := fs.Bool("credits", false, "print remaining credits (free)")
	timeout := fs.Duration("timeout", 15*time.Minute, "how long to wait for the result")
	showVersion := fs.Bool("version", false, "print the version")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: emaki [flags] \"prompt\"\n\nGenerates on https://emaki.ai and prints the file URL. Needs EMAKI_API_KEY (%s).\n\n", keysURL)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, err)
		if e, ok := err.(*apiError); ok {
			switch e.Kind {
			case "auth":
				return 3
			case "quota", "plan":
				return 4
			}
		}
		return 1
	}
	switch {
	case *showVersion:
		fmt.Fprintln(stdout, "emaki", version)
		return 0
	case *showCredits:
		n, err := credits(key)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, n)
		return 0
	case *statusID != "":
		rec, err := status(key, *statusID)
		if err != nil {
			return fail(err)
		}
		out, _ := json.Marshal(rec)
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	prompt := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(prompt) == "" {
		fs.Usage()
		return 2
	}
	params := map[string]any{}
	if *paramsJSON != "" {
		if err := json.Unmarshal([]byte(*paramsJSON), &params); err != nil {
			fmt.Fprintln(stderr, "-params is not a JSON object:", err)
			return 2
		}
	}
	if *duration > 0 {
		params["duration"] = *duration
	}
	if *resolution != "" {
		params["resolution"] = *resolution
	}
	if *aspect != "" {
		params["aspect_ratio"] = *aspect
	}
	if *first != "" {
		params["image"] = *first
	}
	m := *model
	if m == "" {
		m = "seedance_2_5"
		if *image {
			m = "nano_banana_2_lite"
		}
	}
	id, err := submit(key, prompt, m, params)
	if err != nil {
		return fail(err)
	}
	if *noWait {
		fmt.Fprintln(stdout, id)
		return 0
	}
	url, err := wait(key, id, 5*time.Second, *timeout)
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, url)
	return 0
}
