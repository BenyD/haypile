package daemon

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/BenyD/haypile/internal/embed"
	"github.com/BenyD/haypile/internal/index"
	"github.com/BenyD/haypile/internal/ingest"
)

// debounce is how long a path must stay quiet before it is indexed:
// editors and downloads write files in bursts, and indexing a half-written
// document wastes work that the next event redoes anyway.
const debounce = 500 * time.Millisecond

// watcher keeps the index in sync with the filesystem. fsnotify reports
// per-directory, non-recursively, so every directory under every source is
// registered, and new directories are registered as they appear. Events
// are debounced per path, then handed to a small worker pool — indexing
// never blocks event intake, and search (SQLite WAL) never blocks on
// either.
type watcher struct {
	st  *index.Store
	emb embed.Embedder
	fsw *fsnotify.Watcher

	mu      sync.Mutex
	sources map[string]int64       // source root → source id
	timers  map[string]*time.Timer // debounce state per event path
	queued  int                    // jobs debounced or waiting for a worker

	jobs chan job
	done chan struct{}
	wg   sync.WaitGroup
}

type job struct {
	kind     jobKind
	sourceID int64
	path     string // file path (indexFile) or source root (reconcile)
}

type jobKind int

const (
	jobFile      jobKind = iota // one file changed: index just it
	jobReconcile                // something vanished or moved: re-walk the source
)

func newWatcher(st *index.Store, emb embed.Embedder) (*watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &watcher{
		st:      st,
		emb:     emb,
		fsw:     fsw,
		sources: make(map[string]int64),
		timers:  make(map[string]*time.Timer),
		jobs:    make(chan job, 256),
		done:    make(chan struct{}),
	}

	w.wg.Add(1)
	go w.eventLoop()
	// Two workers: enough to overlap extraction with embedding without
	// competing with query traffic for every core.
	for i := 0; i < 2; i++ {
		w.wg.Add(1)
		go w.worker()
	}
	return w, nil
}

func (w *watcher) close() {
	close(w.done)
	w.fsw.Close()
	w.mu.Lock()
	for _, t := range w.timers {
		t.Stop()
	}
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *watcher) pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.queued
}

// watchSource registers a source root (folder or single file) and every
// directory below it.
func (w *watcher) watchSource(root string) error {
	id, err := w.st.SourceID(root)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.sources[root] = id
	w.mu.Unlock()

	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		// Single-file source: watch its parent, filter in eventLoop.
		return w.fsw.Add(filepath.Dir(root))
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return w.fsw.Add(path)
		}
		return nil
	})
}

func (w *watcher) unwatchSource(root string) {
	w.mu.Lock()
	delete(w.sources, root)
	w.mu.Unlock()
	// Watches on directories under root become inert (ownerOf finds no
	// source); fsnotify removes them automatically if the dirs vanish.
}

// ownerOf maps an event path to its source. Longest matching root wins so
// nested sources resolve to the most specific one.
func (w *watcher) ownerOf(path string) (int64, string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var bestRoot string
	var bestID int64
	for root, id := range w.sources {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			if len(root) > len(bestRoot) {
				bestRoot, bestID = root, id
			}
		}
	}
	return bestID, bestRoot, bestRoot != ""
}

func (w *watcher) eventLoop() {
	defer w.wg.Done()
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// Watch errors are transient (unmounts, permission churn);
			// the next reconcile pass repairs any missed state.
		}
	}
}

func (w *watcher) handleEvent(ev fsnotify.Event) {
	id, root, ok := w.ownerOf(ev.Name)
	if !ok {
		return
	}

	// A new directory needs its own watch before events inside it exist.
	if ev.Op.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			w.fsw.Add(ev.Name)
			w.debounced(root, job{kind: jobReconcile, sourceID: id, path: root})
			return
		}
	}

	switch {
	case ev.Op.Has(fsnotify.Remove) || ev.Op.Has(fsnotify.Rename):
		// The old path is gone; a full incremental pass prunes it (and
		// picks up the rename's new name via its own Create event).
		w.debounced(root, job{kind: jobReconcile, sourceID: id, path: root})
	case ev.Op.Has(fsnotify.Create) || ev.Op.Has(fsnotify.Write):
		if !ingest.Supported(ev.Name) {
			return
		}
		w.debounced(ev.Name, job{kind: jobFile, sourceID: id, path: ev.Name})
	}
}

// debounced schedules j to run after the key has been quiet for the
// debounce window, resetting the clock on every new event for that key.
func (w *watcher) debounced(key string, j job) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[key]; ok {
		t.Reset(debounce)
		return
	}
	w.queued++
	w.timers[key] = time.AfterFunc(debounce, func() {
		w.mu.Lock()
		delete(w.timers, key)
		w.mu.Unlock()
		select {
		case w.jobs <- j:
		case <-w.done:
		}
	})
}

func (w *watcher) worker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.done:
			return
		case j := <-w.jobs:
			switch j.kind {
			case jobFile:
				ingest.IndexOne(w.st, j.sourceID, j.path, w.emb)
			case jobReconcile:
				tag, _ := w.st.SourceTag(j.path)
				ingest.IndexFolder(w.st, j.path, tag, w.emb, nil)
			}
			w.mu.Lock()
			w.queued--
			w.mu.Unlock()
		}
	}
}
