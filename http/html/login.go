package html

func (e *Engine) LoginView() (Renderer, error) {
	return e.view("ui/views/login.tmpl.html")
}
