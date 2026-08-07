package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/BenyD/haypile/internal/ingest"
)

// etaTracker estimates time remaining from progress samples. Rate is an
// exponential moving average so one slow PDF does not whipsaw the
// estimate; a phase change resets it because extraction speed says
// nothing about embedding speed.
type etaTracker struct {
	phase    string
	lastDone int64
	lastAt   time.Time
	rate     float64 // units per second, EMA
}

// etaSmoothing weights the newest interval; low enough to hold steady
// through bursty per-file timings.
const etaSmoothing = 0.25

// update feeds one sample and returns the estimate. ok is false until
// there is enough signal to be honest: a phase needs a measured rate
// before "time left" means anything.
func (e *etaTracker) update(phase string, done, total int64, now time.Time) (time.Duration, bool) {
	if phase != e.phase {
		e.phase = phase
		e.lastDone = done
		e.lastAt = now
		e.rate = 0
		return 0, false
	}
	dt := now.Sub(e.lastAt).Seconds()
	if dt <= 0 {
		return e.remaining(done, total)
	}
	sample := float64(done-e.lastDone) / dt
	if e.rate == 0 {
		e.rate = sample
	} else {
		e.rate = etaSmoothing*sample + (1-etaSmoothing)*e.rate
	}
	e.lastDone = done
	e.lastAt = now
	return e.remaining(done, total)
}

func (e *etaTracker) remaining(done, total int64) (time.Duration, bool) {
	if e.rate <= 0 || total <= done {
		return 0, false
	}
	return time.Duration(float64(total-done) / e.rate * float64(time.Second)), true
}

// progressLine renders one live update, feeding the ETA the honest unit
// for the phase: bytes while extracting (file counts lie when one PDF is
// 800 pages), chunks while embedding.
func progressLine(t *etaTracker, p ingest.Progress, now time.Time) string {
	var done, total int64
	if p.Phase == ingest.PhaseEmbedding {
		done, total = int64(p.ChunksDone), int64(p.ChunksTotal)
	} else {
		done, total = p.BytesDone, p.BytesTotal
	}
	eta, ok := t.update(p.Phase, done, total, now)
	return renderProgress(p, eta, ok)
}

// phaseUnits reports what a phase counts, for display. Extraction shows
// files because that is what the user recognises on screen; the ETA reads
// bytes separately, since file counts lie when one PDF is 800 pages.
func phaseUnits(p ingest.Progress) (done, total int64, unit string) {
	if p.Phase == ingest.PhaseEmbedding {
		return int64(p.ChunksDone), int64(p.ChunksTotal), "chunks"
	}
	return int64(p.FilesDone), int64(p.FilesTotal), "files"
}

// milestoneReporter emits one line per tenth of a phase, for output that
// cannot be rewritten in place: pipes, CI logs, and backgrounded runs.
// Those get no live bar at all, so a long embedding pass prints nothing
// between "Indexing …" and the summary — on a large corpus that is an
// hour of silence that reads as a hang.
type milestoneReporter struct {
	phase string
	last  int // last tenth emitted; -1 before the first in a phase
}

// step returns the line to print for this snapshot, or "" when it does
// not cross into a new tenth. A nil snapshot (the pass has not registered
// yet) is not progress, so it prints nothing.
func (m *milestoneReporter) step(p *ingest.Progress) string {
	if p == nil {
		return ""
	}
	if p.Phase != m.phase {
		m.phase, m.last = p.Phase, -1
	}
	done, total, unit := phaseUnits(*p)
	if total <= 0 {
		return ""
	}
	pct := int(done * 100 / total)
	if tenth := pct / 10; tenth > m.last {
		m.last = tenth
		return fmt.Sprintf("  %-10s %3d%% · %s/%s %s",
			p.Phase, pct, count(done), count(total), unit)
	}
	return ""
}

// renderProgress is the one-line live view of an indexing pass:
//
//	extracting [#####·····] 48% · 151/312 files · ~2m left
//	embedding  [##········] 23% · 1.4k/6.2k chunks · ~4m left
func renderProgress(p ingest.Progress, eta time.Duration, haveETA bool) string {
	done, total, unit := phaseUnits(p)
	if total <= 0 {
		return fmt.Sprintf("%s…", p.Phase)
	}
	pct := int(done * 100 / total)
	line := fmt.Sprintf("%-10s %s %3d%% · %s/%s %s",
		p.Phase, bar(pct, 10), pct, count(done), count(total), unit)
	if haveETA {
		line += " · " + fmtETA(eta)
	}
	return line
}

func bar(pct, width int) string {
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("·", width-filled) + "]"
}

// count keeps big chunk numbers readable: 6221 -> 6.2k.
func count(n int64) string {
	if n < 10000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

// fmtETA rounds to a width people read as a promise kept: seconds under
// a minute and a half, whole minutes after, never false precision.
func fmtETA(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return "almost done"
	case d < 90*time.Second:
		return fmt.Sprintf("~%ds left", int(d.Seconds()/5)*5)
	default:
		return fmt.Sprintf("~%dm left", int(d.Minutes()+0.5))
	}
}
