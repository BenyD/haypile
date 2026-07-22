package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// ConfigName is the per-folder configuration file, created by `hay init`
// and editable by hand. The daemon watches it: saving a change reconciles
// the index — newly excluded files drop out, newly included ones come in.
const ConfigName = ".haypile.yml"

// Config is a source folder's indexing configuration.
type Config struct {
	// Tag applies to searches scoped with --tag. A tag passed explicitly
	// to `hay add` wins over this.
	Tag string `yaml:"tag,omitempty"`
	// Exclude lists doublestar patterns (gitignore-flavored globs) matched
	// against paths relative to the folder: "drafts/**", "*.bak",
	// "**/archive/**".
	Exclude []string `yaml:"exclude,omitempty"`
}

// LoadConfig reads root's .haypile.yml. A missing file is a valid empty
// config; a malformed one is an error — silently indexing everything
// against the user's written intent would be worse than failing.
func LoadConfig(root string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(filepath.Join(root, ConfigName))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", ConfigName, err)
	}
	for _, p := range cfg.Exclude {
		if !doublestar.ValidatePattern(p) {
			return cfg, fmt.Errorf("%s: invalid exclude pattern %q", ConfigName, p)
		}
	}
	return cfg, nil
}

// Save writes the config to root's .haypile.yml.
func (c Config) Save(root string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	header := []byte("# Haypile per-folder config — see `hay init --help`.\n# Saving changes re-syncs the index automatically while the daemon runs.\n")
	return os.WriteFile(filepath.Join(root, ConfigName), append(header, data...), 0o644)
}

// Excluded reports whether the path (relative to the config's folder,
// slash-separated) matches any exclude pattern. A bare-name pattern like
// "*.bak" matches at any depth, mirroring gitignore expectations.
// alwaysSkip are directories that hold machine-managed files, never the
// user's documents: dependency trees, build output, virtualenvs, caches.
// Indexing a code project without this rule pulls in thousands of
// dependency READMEs, which buries the user's own writing in search
// results and takes minutes doing it. Excluded() enforces this before
// the user's own patterns; a folder deliberately named node_modules and
// full of real documents is not a case worth serving.
var alwaysSkip = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"venv":         true,
	"__pycache__":  true,
	"target":       true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	"Pods":         true,
	"DerivedData":  true,
}

// SkipDir reports whether a directory name is machine-managed and never
// worth walking into.
func SkipDir(name string) bool { return alwaysSkip[name] }

func (c Config) Excluded(rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, seg := range strings.Split(rel, "/") {
		if alwaysSkip[seg] {
			return true
		}
	}
	for _, p := range c.Exclude {
		if ok, _ := doublestar.Match(p, rel); ok {
			return true
		}
		if !strings.Contains(p, "/") {
			if ok, _ := doublestar.Match(p, filepath.Base(rel)); ok {
				return true
			}
		}
	}
	return false
}
