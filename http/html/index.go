package html

func (e *Engine) IndexView() (Renderer, error) {
	return e.view("ui/views/index.tmpl.html")
}

func (e *Engine) ShortsIndexView() (Renderer, error) {
	return e.view("ui/views/shorts-index.tmpl.html")
}
