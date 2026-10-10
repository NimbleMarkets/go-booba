package main

import (
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	m, err := loadManifest(strings.NewReader(`
groups:
  - title: Bubble Tea in the browser
    demos:
      - name: views
        title: Multiple views
        blurb: The <b>views</b> sample.
        source: ./cmd/booba-view-example
  - title: Terminal Actions (experimental)
    demos:
      - name: actions
        title: Touch controls
        blurb: Help map as buttons.
        source: ./cmd/booba-actions-example
        page: actions
`))
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if len(m.Groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(m.Groups))
	}
	if got := m.Groups[1].Demos[0].Page; got != "actions" {
		t.Errorf("page = %q, want actions", got)
	}
	if got := m.Groups[0].Demos[0].Page; got != "" {
		t.Errorf("page = %q, want empty", got)
	}
	if got := string(m.Groups[0].Demos[0].BlurbHTML()); got != "The <b>views</b> sample." {
		t.Errorf("BlurbHTML = %q", got)
	}
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	_, err := loadManifest(strings.NewReader(validManifestWith("", "") + "        toolchain: tinygo\n"))
	if err == nil {
		t.Fatal("unknown field toolchain should be rejected")
	}
}

func TestLoadManifestRejectsUnsafeDemoNames(t *testing.T) {
	for _, name := range []string{"../escape", "quick/start", "-flag", "Uppercase", "two--hyphens", "trailing-", ""} {
		t.Run(name, func(t *testing.T) {
			_, err := loadManifest(strings.NewReader(validManifestWith("name", name)))
			if err == nil {
				t.Fatal("expected invalid demo name to be rejected")
			}
		})
	}
}

func TestLoadManifestRejectsUnsafeDemoSources(t *testing.T) {
	for _, source := range []string{
		"-toolexec=helper", "cmd/booba-view-example", "/abs/path", "../other",
		"./cmd/../..", "./", "./cmd//x", `./cmd\x`, "./cmd/./x",
	} {
		t.Run(source, func(t *testing.T) {
			_, err := loadManifest(strings.NewReader(validManifestWith("source", source)))
			if err == nil {
				t.Fatal("expected invalid demo source to be rejected")
			}
			if !strings.Contains(err.Error(), "invalid source") {
				t.Fatalf("expected invalid source error, got %v", err)
			}
		})
	}
}

func TestLoadManifestRejectsDuplicateNames(t *testing.T) {
	_, err := loadManifest(strings.NewReader(`
groups:
  - title: A
    demos:
      - { name: dup, title: One, blurb: x, source: ./cmd/booba-view-example }
  - title: B
    demos:
      - { name: dup, title: Two, blurb: y, source: ./cmd/booba-actions-example }
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate names should be rejected, got %v", err)
	}
}

func TestLoadManifestRejectsUnsafePageNames(t *testing.T) {
	for _, page := range []string{"../actions", "Actions", "a/b", "act ions"} {
		t.Run(page, func(t *testing.T) {
			_, err := loadManifest(strings.NewReader(validManifestWith("", "") + "        page: " + page + "\n"))
			if err == nil || !strings.Contains(err.Error(), "invalid page") {
				t.Fatalf("expected invalid page error, got %v", err)
			}
		})
	}
}

func TestLoadManifestRepoManifestPassesValidation(t *testing.T) {
	m, err := loadManifestFile("../../web/demos.yaml")
	if err != nil {
		t.Fatalf("repo manifest should pass validation: %v", err)
	}
	if len(m.AllDemos()) < 2 {
		t.Fatalf("repo manifest should list at least two demos, got %d", len(m.AllDemos()))
	}
}

func TestDemoDerivedValues(t *testing.T) {
	d := Demo{Name: "views", Source: "./cmd/booba-view-example"}
	if got := d.RunCommand(); got != "go run ./cmd/booba-view-example" {
		t.Errorf("RunCommand = %q", got)
	}
	if got := d.SourceURL(); got != "https://github.com/NimbleMarkets/go-booba/tree/main/cmd/booba-view-example" {
		t.Errorf("SourceURL = %q", got)
	}
}

func TestAllDemosFlattens(t *testing.T) {
	m := &Manifest{Groups: []Group{
		{Title: "A", Demos: []Demo{{Name: "x"}, {Name: "y"}}},
		{Title: "B", Demos: []Demo{{Name: "z"}}},
	}}
	got := m.AllDemos()
	if len(got) != 3 || got[0].Name != "x" || got[2].Name != "z" {
		t.Errorf("AllDemos: %+v", got)
	}
}

// validManifestWith returns a one-demo manifest, overriding the given field.
// The returned text ends with a newline after the source line so callers can
// append further demo fields.
func validManifestWith(field, value string) string {
	name, source := "views", "./cmd/booba-view-example"
	switch field {
	case "name":
		name = value
	case "source":
		source = value
	}
	return "groups:\n" +
		"  - title: Demos\n" +
		"    demos:\n" +
		"      - name: " + name + "\n" +
		"        title: Views\n" +
		"        blurb: The views sample.\n" +
		"        source: " + source + "\n"
}
