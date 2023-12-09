package template

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

func ReloaderMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		engine := c.Echo().Renderer
		switch v := engine.(type) {
		case *Template:
			err := engine.(*Template).Register(c.Request().Context(), v.original.funcs, v.original.partials, v.original.pages, v.original.layout)
			if err != nil {
				return err
			}
			return next(c)
		default:
			return fmt.Errorf("unexpected renderer when reloading: %T", engine)
		}
	}
}
