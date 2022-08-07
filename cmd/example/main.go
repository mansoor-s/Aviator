package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mansoor-s/aviator"
	"net/http"
	"path/filepath"
)

func main() {
	viewsAbsPath, err := filepath.Abs("./views")
	if err != nil {
		panic(err)
	}
	assetOutputAbsPath, err := filepath.Abs("./public/bundled_assets")
	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}
	a := aviator.NewAviator(
		aviator.WithViewsPath(viewsAbsPath),
		aviator.WithAssetOutputPath(assetOutputAbsPath),
		aviator.WithDevMode(true),
		aviator.WithNumJsVMs(4),
		aviator.WithStaticAssetRoute("/bundled_assets"),
	)

	err = a.Init()
	if err != nil {
		panic(err)
	}

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Root:   "public",
		Browse: true,
	}))
	//e.Use(middleware.Recover())

	// Routes
	e.GET("/foo", func(c echo.Context) error {
		props := struct {
			Myprop string
		}{
			Myprop: "My Prop Value",
		}
		rendered, err := a.Render(c.Request().Context(), "index.svelte", props)
		if err != nil {
			errStr := ""
			jsErr, ok := err.(aviator.JSError)
			if ok {
				errStr = jsErr.ErrorStackTrace()
			} else {
				errStr = err.Error()
			}
			return c.String(http.StatusOK, errStr)
		}

		return c.HTML(http.StatusOK, rendered)
	})

	e.GET("/a", func(c echo.Context) error {
		props := struct {
			Myprop string
		}{
			Myprop: "My Prop Value",
		}
		rendered, err := a.Render(c.Request().Context(), "frog/tiger.svelte", props)
		if err != nil {
			errStr := ""
			jsErr, ok := err.(aviator.JSError)
			if ok {
				errStr = jsErr.ErrorStackTrace()
			} else {
				errStr = err.Error()
			}
			return c.String(http.StatusOK, errStr)
		}

		return c.HTML(http.StatusOK, rendered)
	})

	assetHandler := a.DynamicAssetHandler("/public/assets/")

	e.GET("/public/assets/:asset", func(e echo.Context) error {
		assetHandler(e.Response(), e.Request())
		return nil
	})

	// Start server
	e.Logger.Fatal(e.Start(":1323"))
	a.Close()
}
