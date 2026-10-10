package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// basePath is the site's path prefix on GitHub Pages.
const basePath = "/go-booba"

// fragmentPath returns the template file for a demo's page fragment.
func fragmentPath(tmplDir, page string) string {
	return filepath.Join(tmplDir, "demos", page+".html.tmpl")
}

// validatePages checks that every demo with a Page has its fragment file.
func validatePages(m *Manifest, tmplDir string) error {
	for _, d := range m.AllDemos() {
		if d.Page == "" {
			continue
		}
		p := fragmentPath(tmplDir, d.Page)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("demo %q: page fragment %s: %w", d.Name, filepath.ToSlash(p), err)
		}
	}
	return nil
}

func renderSite(m *Manifest, tmplDir, outDir string) error {
	if err := validatePages(m, tmplDir); err != nil {
		return err
	}
	parse := func(name string) (*template.Template, error) {
		return template.New("").ParseFiles(
			filepath.Join(tmplDir, "sidebar.html.tmpl"),
			filepath.Join(tmplDir, name),
		)
	}

	indexTmpl, err := parse("index.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse index template: %w", err)
	}
	if err := writeTemplate(indexTmpl, "index.html.tmpl",
		filepath.Join(outDir, "index.html"),
		map[string]any{"Base": basePath, "Manifest": m, "Active": ""}); err != nil {
		return err
	}

	demoTmpl, err := parse("demo.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse demo template: %w", err)
	}
	for _, d := range m.AllDemos() {
		data := map[string]any{"Base": basePath, "Manifest": m, "Active": d.Name, "Demo": d,
			"ExtraBody": template.HTML(""), "ExtraScript": template.HTML("")}
		if d.Page != "" {
			body, script, err := renderFragment(tmplDir, d.Page, data)
			if err != nil {
				return fmt.Errorf("render demo %q fragment %q: %w", d.Name, d.Page, err)
			}
			data["ExtraBody"], data["ExtraScript"] = body, script
		}
		dst := filepath.Join(outDir, "demos", d.Name, "index.html")
		if err := writeTemplate(demoTmpl, "demo.html.tmpl", dst, data); err != nil {
			return fmt.Errorf("render demo %q: %w", d.Name, err)
		}
	}
	return nil
}

// renderFragment executes the "<page>-body" and "<page>-script" templates
// defined in the fragment file. The results are trusted HTML authored in
// this repository.
func renderFragment(tmplDir, page string, data map[string]any) (template.HTML, template.HTML, error) {
	t, err := template.New("").ParseFiles(fragmentPath(tmplDir, page))
	if err != nil {
		return "", "", err
	}
	exec := func(name string) (template.HTML, error) {
		var buf bytes.Buffer
		if err := t.ExecuteTemplate(&buf, name, data); err != nil {
			return "", fmt.Errorf("execute %s: %w", name, err)
		}
		return template.HTML(buf.String()), nil
	}
	body, err := exec(page + "-body")
	if err != nil {
		return "", "", err
	}
	script, err := exec(page + "-script")
	if err != nil {
		return "", "", err
	}
	return body, script, nil
}

func writeTemplate(t *template.Template, entryName, dst string, data any) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := t.ExecuteTemplate(f, entryName, data); err != nil {
		return fmt.Errorf("execute %s: %w", entryName, err)
	}
	return nil
}
