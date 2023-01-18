package html

func (e *Engine) ShortView() (Renderer, error) {
	return e.view("ui/views/short.tmpl.html")
}

func (e *Engine) NewShortView() (Renderer, error) {
	return e.view("ui/views/new-short.tmpl.html")
}
