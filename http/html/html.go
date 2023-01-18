package html

import (
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"

	"github.com/kriive/lil"
)

type Engine struct {
	f  fs.FS
	bt *template.Template
}

type Renderer interface {
	Render(w io.Writer, r *http.Request, data any) error
}

func NewEngine(fs fs.FS) (*Engine, error) {
	if fs == nil {
		fs = FS
	}

	tmpl, err := template.ParseFS(fs,
		"ui/base.tmpl.html",
		"ui/partials/*.tmpl.html",
	)

	if err != nil {
		return nil, err
	}

	return &Engine{
		f:  fs,
		bt: tmpl,
	}, nil
}

type render struct {
	tmpl *template.Template
}

type renderData struct {
	User  *lil.User
	URL   *url.URL
	Data  any
	Flash string
}

func (ren *render) Render(w io.Writer, r *http.Request, data any) error {
	pass := &renderData{
		User:  lil.UserFromContext(r.Context()),
		URL:   r.URL,
		Data:  data,
		Flash: lil.FlashFromContext(r.Context()),
	}

	return ren.tmpl.ExecuteTemplate(w, "base", pass)
}

func newRender(t *template.Template) (*render, error) {
	return &render{
		tmpl: t,
	}, nil
}

func (e *Engine) view(path string) (Renderer, error) {
	tmpl, err := e.bt.Clone()
	if err != nil {
		return nil, err
	}

	t, err := tmpl.ParseFS(e.f, path)
	if err != nil {
		return nil, err
	}

	return &render{
		tmpl: t,
	}, nil
}
