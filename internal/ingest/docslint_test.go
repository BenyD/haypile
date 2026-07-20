package ingest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Docs drift silently: a new format or env var lands in code and the
// reference pages keep describing last month's binary. These tests pin
// the mechanically checkable facts so CI catches the drift instead of a
// user. Prose accuracy still needs eyes; this is the safety net under it.

const repoRoot = "../.."

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

// TestDocsListEverySupportedFormat: the CLI reference must name each
// extension the extractor map actually accepts.
func TestDocsListEverySupportedFormat(t *testing.T) {
	cli := readDoc(t, "website/content/docs/reference/cli.mdx")
	for ext := range extractors {
		if !strings.Contains(cli, "`"+ext+"`") {
			t.Errorf("reference/cli.mdx does not list supported format %s", ext)
		}
	}
}

// TestDocsListEveryEnvVar: every HAYPILE_* variable the code reads must
// appear in the CLI reference's environment table.
func TestDocsListEveryEnvVar(t *testing.T) {
	re := regexp.MustCompile(`HAYPILE_[A-Z_]+`)
	vars := map[string]bool{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, v := range re.FindAllString(string(data), -1) {
				vars[v] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(vars) == 0 {
		t.Fatal("found no HAYPILE_ env vars in source; the scan is broken")
	}

	cli := readDoc(t, "website/content/docs/reference/cli.mdx")
	for v := range vars {
		if !strings.Contains(cli, v) {
			t.Errorf("reference/cli.mdx does not document %s", v)
		}
	}
}

// TestDocsHaveNoEmDashes: em dashes are banned in all user-facing
// Haypile content (README, docs, website copy).
func TestDocsHaveNoEmDashes(t *testing.T) {
	roots := []string{"README.md", "docs", "website/content"}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".md", ".mdx":
			default:
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, "—") {
					t.Errorf("%s:%d contains an em dash", path, i+1)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
