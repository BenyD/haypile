package llm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BenyD/haypile/internal/index"
)

const systemPrompt = `You answer questions using ONLY the numbered context passages provided.
Rules:
- Cite passages inline as [1], [2] wherever you use them.
- If the context does not contain the answer, say so plainly. Never guess.
- Be concise and factual.`

// userPrompt renders the retrieved chunks and question into the RAG
// user message shared by Answer and AnswerStream. The model reads the
// full chunk text: snippets are for result lists, and answering from
// one would starve the model of the passage it is told to cite.
func userPrompt(question string, results []index.Result) string {
	var b strings.Builder
	for i, r := range results {
		passage := r.Text
		if passage == "" {
			passage = r.Snippet
		}
		fmt.Fprintf(&b, "[%d] %s\n%s\n\n", i+1, Citation(r), passage)
	}
	return fmt.Sprintf("Context passages:\n\n%sQuestion: %s", b.String(), question)
}

// Answer runs the generation leg of RAG: retrieved chunks in, cited
// answer out. Retrieval quality is the retriever's job; this function
// makes the model's use of it auditable via the [n] citations.
func Answer(ctx context.Context, c *Client, question string, results []index.Result) (string, error) {
	return c.Chat(ctx, systemPrompt, userPrompt(question, results))
}

// AnswerStream is Answer with the reply delivered token by token.
func AnswerStream(ctx context.Context, c *Client, question string, results []index.Result, onToken func(string) error) error {
	return c.ChatStream(ctx, systemPrompt, userPrompt(question, results), onToken)
}

// Citation renders a source reference: basename plus page for paginated
// formats, basename plus chunk otherwise.
func Citation(r index.Result) string {
	name := filepath.Base(r.Path)
	if r.Page > 0 {
		return fmt.Sprintf("%s, page %d", name, r.Page)
	}
	return fmt.Sprintf("%s, section %d", name, r.Seq+1)
}
