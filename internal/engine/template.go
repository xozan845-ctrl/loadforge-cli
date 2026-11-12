package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tsenart/vegeta/v12/lib"
)

// targetBuilder produces one target per request, rotating data rows and
// expanding {{.Iteration}}, {{.RandomID}} and {{.Column}} placeholders.
type targetBuilder struct {
	cfg      Config
	bodyTmpl string
	keys     []string
	iter     atomic.Uint64
}

func newTargetBuilder(cfg Config) *targetBuilder {
	b := &targetBuilder{cfg: cfg, bodyTmpl: cfg.BodyTemplate}
	if b.bodyTmpl == "" {
		b.bodyTmpl = string(cfg.Body)
	}
	seen := map[string]bool{}
	for _, row := range cfg.Rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				b.keys = append(b.keys, k)
			}
		}
	}
	return b
}

func (b *targetBuilder) build() (vegeta.Target, error) {
	n := b.iter.Add(1) - 1
	row := map[string]string{}
	if len(b.cfg.Rows) > 0 {
		row = b.cfg.Rows[int(n)%len(b.cfg.Rows)]
	}

	body := b.bodyTmpl
	if body != "" {
		repl := make([]string, 0, 2*(len(b.keys)+2))
		for _, k := range b.keys {
			repl = append(repl, "{{."+k+"}}", row[k])
		}
		repl = append(repl,
			"{{.Iteration}}", fmt.Sprintf("%d", n+1),
			"{{.RandomID}}", randomID(),
		)
		body = strings.NewReplacer(repl...).Replace(body)
	}

	headers := b.cfg.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	return vegeta.Target{
		Method: b.cfg.Method,
		URL:    b.cfg.URL,
		Body:   []byte(body),
		Header: headers,
	}, nil
}

func randomID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(buf[:])
}
