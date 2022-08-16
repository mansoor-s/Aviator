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
		aviator.WithNumJsVMs(8),
		aviator.WithStaticAssetRoute("/public/assets/"),
	)

	err = a.Init()
	if err != nil {
		panic(err.Error())
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
		rendered, err := a.Render(c.Request().Context(), "Index.svelte", props)
		if err != nil {
			return c.String(http.StatusOK, err.Error())
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

			return c.String(http.StatusOK, err.Error())
		}

		return c.HTML(http.StatusOK, rendered)
	})

	e.GET("/public/assets/:asset", func(e echo.Context) error {
		asset, mime, found := a.GetStaticAsset(e.Param("asset"))
		if found {
			return e.Blob(http.StatusOK, mime, asset)
		}
		return e.String(http.StatusNotFound, "not found!")
	})

	// Start server
	e.Logger.Fatal(e.Start(":1323"))
}
