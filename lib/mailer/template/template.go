// Package template loads mail bodies from an fs.FS (typically an embed.FS)
// into a small in-memory registry and renders them by name. A file named
// <name>.txt is a text/template body; <name>.html is an html/template body
// (contextual escaping). A name may have either or both.
//
// Example usage:
//
//	//go:embed *.txt *.html
//	var files embed.FS
//
//	reg, err := template.New(files)
//	text, html, err := reg.Render("welcome", map[string]string{"Name": "Bia"})
package template

import (
	"bytes"
	htmltemplate "html/template"
	"io/fs"
	"strings"
	texttemplate "text/template"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Registry holds parsed templates by name. It is read-only after New, so it
// is safe for concurrent use.
type Registry struct {
	text map[string]*texttemplate.Template
	html map[string]*htmltemplate.Template
}

// New parses every *.txt and *.html file at the root of fsys (use fs.Sub for
// a subdirectory). It fails on the first template that does not parse.
func New(fsys fs.FS) (*Registry, error) {
	r := &Registry{text: map[string]*texttemplate.Template{}, html: map[string]*htmltemplate.Template{}}
	txt, err := fs.Glob(fsys, "*.txt")
	if err != nil {
		return nil, loadErr(err)
	}
	for _, f := range txt {
		t, err := texttemplate.ParseFS(fsys, f)
		if err != nil {
			return nil, loadErr(err)
		}
		r.text[strings.TrimSuffix(f, ".txt")] = t
	}
	html, err := fs.Glob(fsys, "*.html")
	if err != nil {
		return nil, loadErr(err)
	}
	for _, f := range html {
		t, err := htmltemplate.ParseFS(fsys, f)
		if err != nil {
			return nil, loadErr(err)
		}
		r.html[strings.TrimSuffix(f, ".html")] = t
	}
	return r, nil
}

// Render executes the text and HTML bodies of name with data. A body that
// does not exist renders as "". An unknown name returns an
// errorz.ErrNotFound error.
func (r *Registry) Render(name string, data any) (text, html string, err error) {
	tt, okText := r.text[name]
	ht, okHTML := r.html[name]
	if !okText && !okHTML {
		return "", "", errorz.NotFound().WithMessage("template: no template named " + name)
	}
	var b bytes.Buffer
	if okText {
		if err := tt.Execute(&b, data); err != nil {
			return "", "", renderErr(name, err)
		}
		text = b.String()
		b.Reset()
	}
	if okHTML {
		if err := ht.Execute(&b, data); err != nil {
			return "", "", renderErr(name, err)
		}
		html = b.String()
	}
	return text, html, nil
}

func loadErr(err error) error {
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("template: load failed")
}

func renderErr(name string, err error) error {
	return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("template: render " + name + " failed")
}
