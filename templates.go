package main

import (
	"html/template"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/multitemplate"
)

func loadTemplates(templatesDir string) multitemplate.Renderer {
	renderer := multitemplate.NewRenderer()

	funcMap := template.FuncMap{
		"hasPrefix": strings.HasPrefix,
	}

	layouts, err := filepath.Glob(templatesDir + "/layouts/*.html")
	if err != nil {
		panic(err.Error())
	}

	partials, err := filepath.Glob(templatesDir + "/partials/*.html")
	if err != nil {
		panic(err.Error())
	}

	rootIncludes, err := filepath.Glob(templatesDir + "/*.html")
	if err != nil {
		panic(err.Error())
	}

	nestedIncludes, err := filepath.Glob(templatesDir + "/*/*.html")
	if err != nil {
		panic(err.Error())
	}

	includes := append(rootIncludes, nestedIncludes...)

	for _, include := range includes {
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)

		files := append(layoutCopy, include)
		files = append(files, partials...)

		relPath, err := filepath.Rel(templatesDir, include)
		if err != nil {
			panic(err.Error())
		}
		name := filepath.ToSlash(relPath)
		renderer.AddFromFilesFuncs(name, funcMap, files...)
	}

	return renderer
}
