package render

import (
	"context"
)

type RenderOptionsFunc func(*RenderOptions)
type RenderOptions struct {
	ctx      context.Context
	title    string
	data     any
	template string

	// Error handling
	err         error
	errors      []map[string]any
	errorStatus int
	errorCode   string
	errorData   any

	//authentication
	withoutAuthentication bool
}

func defaultRenderOptions() RenderOptions {
	return RenderOptions{
		ctx:                   context.Background(),
		title:                 "Home",
		err:                   nil,
		errorStatus:           0,
		errorCode:             "",
		errorData:             nil,
		withoutAuthentication: false,
	}
}

func (app *Render) NewRenderOptions(opts ...RenderOptionsFunc) *RenderOptions {
	o := defaultRenderOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return &o
}

func (r *Render) WithContext(ctx context.Context) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.ctx = ctx
	}
}
func (r *Render) WithStatus(found int) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.errorStatus = found
	}
}

// WithTitle returns a functional option that sets the title for the render.
//
// Example:
//
//	return app.View(c, app.WithTitle("Hello, World!"))
func (app *Render) WithTitle(title string) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.title = title
	}
}

func (app *Render) WithData(data any) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.data = data
	}
}

func (app *Render) WithError(err error) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.err = err
	}
}

func (app *Render) WithErrorCode(code string) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.errorCode = code
	}
}

func (app *Render) WithErrorData(data any) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.errorData = data
	}
}

func (app *Render) ErrorField(field string, message string) map[string]any {
	return map[string]any{
		"field":   field,
		"message": message,
	}
}

func (app *Render) ErrorFields(fields ...map[string]any) []map[string]any {
	return fields
}
