package llm

import (
	"strings"
	"testing"

	"github.com/BenyD/haypile/internal/index"
)

// The answering model reads full chunks. Snippets are 160-character
// display excerpts; a model fed those says "not in the context" about
// facts the document plainly contains.
func TestUserPromptCarriesFullChunkText(t *testing.T) {
	full := "Education\nHindustan Institute of Technology and Science\nB.Tech in Computer Science Engineering (GPA: 8.5)"
	r := index.Result{Path: "/docs/resume.pdf", Page: 1, Snippet: "… Education Hi …", Text: full}

	prompt := userPrompt("what degree does the resume mention?", []index.Result{r})
	if !strings.Contains(prompt, "B.Tech in Computer Science Engineering") {
		t.Fatalf("prompt must contain the full chunk text, got:\n%s", prompt)
	}

	// Older callers without Text still work from the snippet.
	r.Text = ""
	prompt = userPrompt("q", []index.Result{r})
	if !strings.Contains(prompt, "… Education Hi …") {
		t.Fatalf("prompt must fall back to the snippet, got:\n%s", prompt)
	}
}
