package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// repoURL is where demo source links point.
const repoURL = "https://github.com/NimbleMarkets/go-booba/tree/main/"

type Manifest struct {
	Groups []Group `yaml:"groups"`
}

type Group struct {
	Title string `yaml:"title"`
	Demos []Demo `yaml:"demos"`
}

type Demo struct {
	Name   string `yaml:"name"`
	Title  string `yaml:"title"`
	Blurb  string `yaml:"blurb"`
	Source string `yaml:"source"`
	// Page names an optional fragment template under web/_templates/demos/
	// that adds markup and a setup script to this demo's page.
	Page string `yaml:"page"`
}

// BlurbHTML returns the blurb as trusted HTML. Blurbs are authored in this
// repository's manifest and may use inline markup such as <kbd>.
func (d Demo) BlurbHTML() template.HTML { return template.HTML(d.Blurb) }

// RunCommand is the native run command shown on the demo page. The module
// carries a replace directive, so demos run from a checkout rather than
// through pkg@version.
func (d Demo) RunCommand() string { return "go run " + d.Source }

// SourceURL links to the demo's source directory on GitHub.
func (d Demo) SourceURL() string { return repoURL + strings.TrimPrefix(d.Source, "./") }

func (m *Manifest) AllDemos() []Demo {
	var out []Demo
	for _, g := range m.Groups {
		out = append(out, g.Demos...)
	}
	return out
}

func loadManifest(r io.Reader) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if len(m.Groups) == 0 {
		return nil, fmt.Errorf("manifest has no groups")
	}
	seen := map[string]string{}
	for gi, g := range m.Groups {
		if g.Title == "" {
			return nil, fmt.Errorf("group %d: title required", gi)
		}
		for di, d := range g.Demos {
			if d.Name == "" || d.Title == "" || d.Source == "" {
				return nil, fmt.Errorf("group %q demo %d: name/title/source required", g.Title, di)
			}
			if !validSlug(d.Name) {
				return nil, fmt.Errorf("group %q demo %d: invalid name %q: use lowercase letters, digits, and single hyphens", g.Title, di, d.Name)
			}
			if prev, dup := seen[d.Name]; dup {
				return nil, fmt.Errorf("group %q demo %q: duplicate name, already used in group %q", g.Title, d.Name, prev)
			}
			seen[d.Name] = g.Title
			if !validDemoSource(d.Source) {
				return nil, fmt.Errorf("group %q demo %q: invalid source %q: use a ./path inside the repository without traversal", g.Title, d.Name, d.Source)
			}
			if d.Page != "" && !validSlug(d.Page) {
				return nil, fmt.Errorf("group %q demo %q: invalid page %q: use lowercase letters, digits, and single hyphens", g.Title, d.Name, d.Page)
			}
		}
	}
	return &m, nil
}

// validSlug accepts lowercase letters, digits, and single interior hyphens.
func validSlug(s string) bool {
	if s == "" {
		return false
	}
	prevHyphen := true // rejects a leading hyphen
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevHyphen = false
		case r == '-':
			if prevHyphen {
				return false
			}
			prevHyphen = true
		default:
			return false
		}
	}
	return !prevHyphen
}

// validDemoSource accepts "./a/b" style paths that stay inside the repository.
func validDemoSource(source string) bool {
	if !strings.HasPrefix(source, "./") || strings.Contains(source, `\`) {
		return false
	}
	rest := strings.TrimPrefix(source, "./")
	if rest == "" {
		return false
	}
	for _, part := range strings.Split(rest, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func loadManifestFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return loadManifest(f)
}
