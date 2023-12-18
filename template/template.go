package template

import (
	"fmt"
	"io"
	"strings"

	"context"
	"html/template"
	"path/filepath"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/tcghouse/lib/trace"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("templateutil")

type Template struct {
	templates   map[string]*template.Template
	pagesDir    string
	partialsDir string
	original    struct {
		pages    string
		partials string
		layout   string
		funcs    template.FuncMap
	}
	mutex sync.RWMutex
}

func New() *Template {
	return &Template{
		templates: make(map[string]*template.Template, 0),
	}
}

func (t *Template) Register(ctx context.Context, funcs template.FuncMap, partials, pages, layout string) error {
	_, span := trace.TraceFunction(ctx, tracer)
	defer span.End()

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// save copies of inputs for reloader middleware
	t.original.layout = layout
	t.original.pages = pages
	t.original.partials = partials
	t.original.funcs = funcs

	t.pagesDir = filepath.Dir(pages)
	t.partialsDir = filepath.Dir(partials)

	partialMatches, err := filepath.Glob(partials)
	if err != nil {
		return err
	}

	pageMatches, err := filepath.Glob(pages)
	if err != nil {
		return err
	}

	for _, match := range pageMatches {
		name := filepath.Base(match)
		templateFiles := append(partialMatches, match, layout)
		t.templates[match], err = template.New(name).Funcs(funcs).ParseFiles(templateFiles...)
		if err != nil {
			return err
		}
	}

	t.templates["partials"], err = template.New("partials").Funcs(funcs).ParseFiles(partialMatches...)
	if err != nil {
		return err
	}

	return nil
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	_, span := trace.TraceFunction(c.Request().Context(), tracer)
	defer span.End()

	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if strings.HasPrefix(name, t.pagesDir) {
		splitName := strings.Split(name, "#")

		tmpl, ok := t.templates[splitName[0]]
		if !ok {
			return fmt.Errorf("unable to find template: %q", name)
		}

		// render a partial
		if len(splitName) > 1 {
			err := tmpl.ExecuteTemplate(w, splitName[1], data)
			if err != nil {
				err = fmt.Errorf("tried to execute partial %q in %q: %w", splitName[1], splitName[0], err)
			}
			return err
		}

		// render a page
		err := tmpl.ExecuteTemplate(w, "base", data)
		if err != nil {
			err = fmt.Errorf("tried to execute \"base\" for %q: %w", splitName[0], err)
		}
		return err
	}

	err := t.templates["partials"].ExecuteTemplate(w, name, data)
	if err != nil {
		err = fmt.Errorf("tried to execute pure partial: %w", err)
	}
	return err

}
