package builder

import (
	"github.com/mansoor-s/aviator/utils"
	"path/filepath"
	"unicode"
)

// View objects are passed to the go-template file responsible for creating the virtual __aviator_ssr.js file
// in the SSR Plugin
// A View object can represent both a Component or a Layout
type View struct {
	ComponentName string

	//UniqueName is the PascalCase of the Path relative to views directory
	UniqueName string

	//WrappedUniqueName prefixes the unique name with "__AviatorWrapped_"
	//i.e __AviatorWrapped{UniqueName}
	//This is used to disambiguate the layout wrapped component from the component itself
	WrappedUniqueName string
	Path              string

	//RelPath is the relative Path from the project's views directory
	RelPath string

	//A view can represent either a Component or a Layout
	Component *Component
	Layout    *Layout

	IsLayout bool

	//If the view is a svelte Component and starts with a capital letter, it will be
	//treated as an entrypoint for both SSR and Browser JS
	IsEntrypoint bool

	//ApplicableLayouts is a slice of Views that represent layouts that apply to this
	//view. Lower index means the layout is closer to this view in the ancestral hierarchy
	ApplicableLayoutViews []*View

	//the imports are generated during the browser build step and are injected into
	// the HTML at render time
	JSImports  []string
	CSSImports []string

	//applicableLayouts is used temporarily internally by viewManger
	applicableLayouts []*Layout
}

func (v *View) getApplicableLayouts() []*Layout {
	var layouts []*Layout
	if v.IsLayout {
		layouts = v.Layout.ApplicableLayouts()
	} else {
		layouts = v.Component.ApplicableLayouts()
	}

	return layouts
}

func newViewFromComponent(c *Component) *View {
	fileName := filepath.Base(c.Path)
	firstRune := []rune(fileName)[0]
	var isEntrypoint bool
	if unicode.IsUpper(firstRune) && unicode.IsLetter(firstRune) {
		isEntrypoint = true
	}

	uniqueName := utils.PathPascalCase(c.RelativePath())
	return &View{
		Path:              c.Path,
		RelPath:           c.RelativePath(),
		UniqueName:        uniqueName,
		WrappedUniqueName: "__AviatorWrapped_" + uniqueName,
		ComponentName:     c.Name,
		Component:         c,
		Layout:            c.Layout,
		IsEntrypoint:      isEntrypoint,
	}
}

func newViewFromLayout(l *Layout) *View {
	uniqueName := utils.PathPascalCase(l.RelativePath())
	return &View{
		Path:              l.Path,
		RelPath:           l.RelativePath(),
		UniqueName:        uniqueName,
		WrappedUniqueName: "__AviatorWrapped_" + uniqueName,
		ComponentName:     l.Name,
		Layout:            l,
		IsLayout:          true,
	}
}
