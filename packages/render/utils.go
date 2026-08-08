package render

func staticErrorText(err error) string {
	return err.Error()
}

func (r *Render) prepareThemeString() string {
	templateData, err := r.assets.ReadFile("static/index.html")
	if err != nil {
		return staticErrorText(err)
	}

	return string(templateData)
}
