package engine

import (
	"strings"
	"testing"
)

func TestTargetBuilderTemplatesRowsAndIteration(t *testing.T) {
	cfg := Config{
		Method:       "POST",
		URL:          "http://example.test",
		BodyTemplate: `{"id":{{.Iteration}},"name":"{{.name}}","rid":"{{.RandomID}}"}`,
		Rows: []map[string]string{
			{"name": "uno"},
			{"name": "dos"},
		},
	}
	b := newTargetBuilder(cfg)

	first, err := b.build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	body := string(first.Body)
	if !strings.Contains(body, `"id":1`) || !strings.Contains(body, `"name":"uno"`) {
		t.Fatalf("unexpected first body: %s", body)
	}
	if strings.Contains(body, "{{") {
		t.Fatalf("unexpanded placeholder: %s", body)
	}

	second, _ := b.build()
	if !strings.Contains(string(second.Body), `"id":2`) ||
		!strings.Contains(string(second.Body), `"name":"dos"`) {
		t.Fatalf("unexpected second body: %s", second.Body)
	}
}

func TestTargetBuilderRotatesRows(t *testing.T) {
	cfg := Config{
		Method:       "GET",
		URL:          "http://example.test",
		BodyTemplate: "{{.v}}",
		Rows:         []map[string]string{{"v": "a"}, {"v": "b"}},
	}
	b := newTargetBuilder(cfg)
	var got []string
	for i := 0; i < 4; i++ {
		target, _ := b.build()
		got = append(got, string(target.Body))
	}
	if strings.Join(got, "") != "abab" {
		t.Fatalf("rows = %v, want abab", got)
	}
}
