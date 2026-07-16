package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
)

// Client talks to a running daemon over its localhost API.
type Client struct {
	base string
	http *http.Client
}

// Discover returns a client for the daemon serving this HAYPILE_DIR, or
// nil when none is running. A daemon serving a different database is
// treated as absent — routing a query to the wrong index would be worse
// than a slow direct query.
func Discover() *Client {
	dataDir := filepath.Dir(index.DefaultPath())
	rt, err := readRuntimeFile(dataDir)
	if err != nil {
		return nil
	}
	c := &Client{
		base: "http://" + rt.Addr,
		http: &http.Client{Timeout: 10 * time.Minute}, // indexing a big folder is slow
	}
	h, err := c.health()
	if err != nil || !h.OK || h.DB != index.DefaultPath() {
		return nil
	}
	return c
}

// AutoStart returns a client, launching the daemon first if none is
// running. HAYPILE_NO_DAEMON=1 disables it (tests, scripting); nil with
// no error means "proceed without a daemon".
func AutoStart() (*Client, error) {
	if os.Getenv("HAYPILE_NO_DAEMON") == "1" {
		return nil, nil
	}
	if c := Discover(); c != nil {
		return c, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, nil
	}
	cmd := exec.Command(exe, "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("auto-starting daemon: %w", err)
	}
	// The child outlives us; Release avoids a zombie until then.
	cmd.Process.Release()

	// The daemon is up once its runtime file appears and health answers.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c := Discover(); c != nil {
			return c, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, nil // slow start: caller proceeds direct, daemon catches up
}

func (c *Client) health() (Health, error) {
	var h Health
	// Health must answer fast even when the daemon is busy indexing.
	quick := &http.Client{Timeout: 2 * time.Second}
	resp, err := quick.Get(c.base + "/api/health")
	if err != nil {
		return h, err
	}
	defer resp.Body.Close()
	return h, json.NewDecoder(resp.Body).Decode(&h)
}

// Status fetches the daemon's full status report.
func (c *Client) Status() (Status, error) {
	var st Status
	return st, c.get("/api/status", &st)
}

// Query runs a search through the daemon (warm model, fresh index).
func (c *Client) Query(q, tag string, limit int) ([]index.Result, error) {
	var out struct {
		Results []QueryResult `json:"results"`
	}
	err := c.post("/api/query", QueryRequest{Query: q, Tag: tag, Limit: limit}, &out)
	if err != nil {
		return nil, err
	}
	results := make([]index.Result, len(out.Results))
	for i, r := range out.Results {
		results[i] = index.Result{
			Path: r.Path, Page: r.Page, Seq: r.Chunk,
			Snippet: r.Snippet, Score: r.Score,
		}
	}
	return results, nil
}

// AddSource indexes and watches a folder or file.
func (c *Client) AddSource(path, tag string) (ingest.Stats, error) {
	var stats ingest.Stats
	abs, err := filepath.Abs(path)
	if err != nil {
		return stats, err
	}
	return stats, c.post("/api/sources", AddSourceRequest{Path: abs, Tag: tag}, &stats)
}

// RemoveSource un-indexes and un-watches a source.
func (c *Client) RemoveSource(path string) (bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	req, err := json.Marshal(AddSourceRequest{Path: abs})
	if err != nil {
		return false, err
	}
	httpReq, err := http.NewRequest(http.MethodDelete, c.base+"/api/sources", bytes.NewReader(req))
	if err != nil {
		return false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var out struct {
		Removed bool `json:"removed"`
	}
	return out.Removed, decodeResponse(resp, &out)
}

// Sources lists indexed sources through the daemon.
func (c *Client) Sources() ([]SourceInfo, error) {
	var out struct {
		Sources []SourceInfo `json:"sources"`
	}
	return out.Sources, c.get("/api/sources", &out)
}

func (c *Client) get(path string, v any) error {
	resp, err := c.http.Get(c.base + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, v)
}

func (c *Client) post(path string, body, v any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.http.Post(c.base+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, v)
}

func decodeResponse(resp *http.Response, v any) error {
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error string `json:"error"`
		}
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if json.Unmarshal(msg, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("daemon: %s", apiErr.Error)
		}
		return fmt.Errorf("daemon returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
