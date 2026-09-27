// Package typing turns classified key presses into typing statistics.
//
// Only counts are kept. The shadow buffer used to handle corrections stores
// run lengths (how long the current word or gap is), never the characters.
package typing

import (
	"time"

	"github.com/ayanrajpoot10/hyprtime/internal/db"
)

type Kind int

const (
	// Char is a printable character that isn't a word separator
	Char Kind = iota
	// Separator ends a word: space, enter or tab
	Separator
	// Backspace deletes one character
	Backspace
	// WordBackspace deletes the previous word (ctrl+backspace)
	WordBackspace
	// Reset moves the caret or runs a shortcut, so the shadow buffer no
	// longer matches the text on screen
	Reset
)

type Key struct {
	Kind Kind
	Time time.Time
}

const (
	// A pause this long ends a typing burst
	BurstGap = 2 * time.Second
	// Bursts shorter than this aren't used for speed
	minBurstChars    = 10
	minBurstDuration = time.Second
	// Deleting a word we have no record of counts as an average word
	unknownWordLen = 5
	maxRuns        = 256
)

type run struct {
	word bool
	len  int
}

// Snapshot is the live state published for status bars.
type Snapshot struct {
	Date       string  `json:"date"`
	LastKeyMs  int64   `json:"last_key_ms"`
	Typing     bool    `json:"typing"`
	LiveWPM    float64 `json:"live_wpm"`
	LastWPM    float64 `json:"last_burst_wpm"`
	LastAcc    float64 `json:"last_burst_accuracy"`
	Chars      int64   `json:"chars"`
	Backspaces int64   `json:"backspaces"`
	Words      int64   `json:"words"`
	ActiveMs   int64   `json:"active_ms"`
	BurstChars int64   `json:"burst_chars"`
	BurstBack  int64   `json:"burst_backspaces"`
	BurstMs    int64   `json:"burst_ms"`
	BestWPM    float64 `json:"best_wpm"`
}

type Counter struct {
	runs []run

	inBurst    bool
	burstApp   int64
	burstStart time.Time
	lastKey    time.Time
	burstChars int64
	burstBack  int64

	pending map[int64]*db.TypingTotals
	bursts  []db.Burst

	date    string
	today   db.TypingTotals
	todayB  db.BurstTotals
	lastWPM float64
	lastAcc float64
}

func NewCounter() *Counter {
	return &Counter{pending: map[int64]*db.TypingTotals{}}
}

// Restore seeds today's totals from the database after a restart.
func (c *Counter) Restore(date string, t db.TypingTotals, b db.BurstTotals) {
	c.date = date
	c.today = t
	c.todayB = b
}

func (c *Counter) totals(app int64) *db.TypingTotals {
	p, ok := c.pending[app]
	if !ok {
		p = &db.TypingTotals{}
		c.pending[app] = p
	}
	return p
}

func (c *Counter) rollDate(now time.Time) {
	date := now.Format("2006-01-02")
	if date != c.date {
		c.date = date
		c.today = db.TypingTotals{}
		c.todayB = db.BurstTotals{}
	}
}

// Handle counts one key press for the given app.
func (c *Counter) Handle(k Key, app int64) {
	c.rollDate(k.Time)

	if c.inBurst && (app != c.burstApp || k.Time.Sub(c.lastKey) > BurstGap) {
		c.endBurst()
	}

	if k.Kind == Reset {
		c.runs = nil
		return
	}

	if !c.inBurst {
		c.inBurst = true
		c.burstApp = app
		c.burstStart = k.Time
		c.burstChars = 0
		c.burstBack = 0
	}
	c.lastKey = k.Time

	p := c.totals(app)

	switch k.Kind {
	case Char, Separator:
		p.Chars++
		c.today.Chars++
		c.burstChars++
		if k.Kind == Separator {
			if c.push(false) {
				p.Words++
				c.today.Words++
			}
		} else {
			c.push(true)
		}

	case Backspace:
		// Counts even with nothing in the buffer: text typed before the
		// buffer was reset is still being corrected
		_, words := c.removeOne()
		p.Backspaces++
		c.today.Backspaces++
		c.burstBack++
		p.Words -= words
		c.today.Words -= words

	case WordBackspace:
		n, words := c.removeWord()
		if n == 0 {
			n = unknownWordLen
		}
		p.Backspaces += int64(n)
		c.today.Backspaces += int64(n)
		c.burstBack += int64(n)
		p.Words -= words
		c.today.Words -= words
	}
}

// push appends a character to the shadow buffer. It reports whether the
// character completed a word (a separator directly after a word).
func (c *Counter) push(word bool) bool {
	n := len(c.runs)
	if n > 0 && c.runs[n-1].word == word {
		c.runs[n-1].len++
		return false
	}

	completes := !word && n > 0 && c.runs[n-1].word
	c.runs = append(c.runs, run{word: word, len: 1})
	if len(c.runs) > maxRuns {
		c.runs = c.runs[len(c.runs)-maxRuns:]
	}
	return completes
}

// removeOne deletes the last character from the shadow buffer. It reports
// whether there was one, and how many completed words that undid.
func (c *Counter) removeOne() (bool, int64) {
	n := len(c.runs)
	if n == 0 {
		return false, 0
	}

	last := &c.runs[n-1]
	last.len--
	if last.len > 0 {
		return true, 0
	}

	c.runs = c.runs[:n-1]
	// Removing the whole gap after a word makes that word incomplete again
	if !last.word && n > 1 && c.runs[n-2].word {
		return true, 1
	}
	return true, 0
}

// removeWord deletes trailing separators and then the word before them,
// like ctrl+backspace in most editors.
func (c *Counter) removeWord() (int, int64) {
	var n int
	var words int64

	for len(c.runs) > 0 && !c.runs[len(c.runs)-1].word {
		_, w := c.removeOne()
		n++
		words += w
	}
	for len(c.runs) > 0 && c.runs[len(c.runs)-1].word {
		c.removeOne()
		n++
	}
	return n, words
}

// ResetBuffer forgets the shadow buffer, e.g. after focus changes.
func (c *Counter) ResetBuffer() {
	c.runs = nil
}

// Tick ends a burst once the typing pause is long enough. It reports
// whether a burst ended.
func (c *Counter) Tick(now time.Time) bool {
	c.rollDate(now)
	if c.inBurst && now.Sub(c.lastKey) > BurstGap {
		c.endBurst()
		return true
	}
	return false
}

func (c *Counter) endBurst() {
	c.inBurst = false
	c.runs = nil

	dur := c.lastKey.Sub(c.burstStart)
	p := c.totals(c.burstApp)
	p.ActiveMs += dur.Milliseconds()
	c.today.ActiveMs += dur.Milliseconds()

	if c.burstChars < minBurstChars || dur < minBurstDuration {
		return
	}

	c.bursts = append(c.bursts, db.Burst{
		Start:      c.burstStart,
		AppID:      c.burstApp,
		DurationMs: dur.Milliseconds(),
		Chars:      c.burstChars,
		Backspaces: c.burstBack,
	})

	c.todayB.Chars += c.burstChars
	c.todayB.Backspaces += c.burstBack
	c.todayB.DurationMs += dur.Milliseconds()

	c.lastWPM = WPM(c.burstChars, c.burstBack, dur.Milliseconds())
	c.lastAcc = Accuracy(c.burstChars, c.burstBack)
	if c.lastWPM > c.todayB.BestWPM {
		c.todayB.BestWPM = c.lastWPM
	}
}

// Finish ends any burst in progress, e.g. on shutdown.
func (c *Counter) Finish() {
	if c.inBurst {
		c.endBurst()
	}
}

// Drain hands over everything not yet written to the database.
func (c *Counter) Drain() (map[int64]db.TypingTotals, []db.Burst) {
	totals := make(map[int64]db.TypingTotals, len(c.pending))
	for app, p := range c.pending {
		totals[app] = *p
	}
	bursts := c.bursts

	c.pending = map[int64]*db.TypingTotals{}
	c.bursts = nil
	return totals, bursts
}

func (c *Counter) Snapshot(now time.Time) Snapshot {
	s := Snapshot{
		Date:       c.date,
		LastWPM:    round1(c.lastWPM),
		LastAcc:    round1(c.lastAcc),
		Chars:      c.today.Chars,
		Backspaces: c.today.Backspaces,
		Words:      max(c.today.Words, 0),
		ActiveMs:   c.today.ActiveMs,
		BurstChars: c.todayB.Chars,
		BurstBack:  c.todayB.Backspaces,
		BurstMs:    c.todayB.DurationMs,
		BestWPM:    round1(c.todayB.BestWPM),
	}
	if !c.lastKey.IsZero() {
		s.LastKeyMs = c.lastKey.UnixMilli()
	}
	if c.inBurst && now.Sub(c.lastKey) <= BurstGap {
		s.Typing = true
		if elapsed := c.lastKey.Sub(c.burstStart); elapsed >= minBurstDuration {
			s.LiveWPM = round1(WPM(c.burstChars, c.burstBack, elapsed.Milliseconds()))
		}
	}
	return s
}

// WPM uses the standard five characters per word, net of corrections.
func WPM(chars, backspaces, ms int64) float64 {
	if ms <= 0 {
		return 0
	}
	return float64(max(chars-backspaces, 0)) / 5 / (float64(ms) / 60000)
}

// Accuracy is the share of typed characters that weren't deleted again.
func Accuracy(chars, backspaces int64) float64 {
	if chars <= 0 {
		return 100
	}
	return float64(max(chars-backspaces, 0)) / float64(chars) * 100
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
