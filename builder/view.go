package builder

import "github.com/mansoor-s/aviator/utils"

// View objects are passed to the go-template file responsible for creating the virtual __aviator_ssr.js file
// in the SSR Plugin
// During render time, their UniqueName and
// A View object can represent both a Component or a Layout
type View struct {
	ComponentName string
	//UniqueName is the PascalCase of the path relative to views directory
	UniqueName string
	//WrappedUniqueName prefixes the unique name with "__AviatorWrapped_"
	//i.e __AviatorWrapped{UniqueName}
	//This is used to disambiguate the layout wrapped component from the component itself
	WrappedUniqueName string
	Path              string
	//RelPath is the relative path from the project's views directory
	RelPath string

	//A view can represent either a Component or a Layout
	Component *Component
	Layout    *Layout

	IsLayout bool

	//ApplicableLayouts is a slice of Views that represent layouts that apply to this
	//view. Lower index means the layout is closer to this view in the ancestral hierarchy
	ApplicableLayoutViews []*View

	//the imports are generated during the browser build step and are injected into
	// the HTML at render time
	JSImports  []string
	CSSImports []string

	//applicableLayouts is used temporarily internally by viewManger
	applicableLayouts []*Layout

	//TODO: when Browser building is finished
	ClientCode string
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
	uniqueName := utils.PathPascalCase(c.RelativePath())
	return &View{
		Path:              c.Path,
		RelPath:           c.RelativePath(),
		UniqueName:        uniqueName,
		WrappedUniqueName: "__AviatorWrapped_" + uniqueName,
		ComponentName:     c.Name,
		Component:         c,
		Layout:            c.Layout,
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

//ViewManager stores views and fetches them during build and render time
type ViewManager struct {
	views map[string]*View
}

func NewViewManager(tree ComponentTree) *ViewManager {
	views := make(map[string]*View)
	for _, component := range tree.GetAllComponents() {
		view := newViewFromComponent(component)
		view.applicableLayouts = component.ApplicableLayouts()
		views[component.RelativePath()] = view
	}

	for _, layout := range tree.GetAllLayouts() {
		view := newViewFromLayout(layout)
		view.applicableLayouts = layout.ApplicableLayouts()
		views[layout.RelativePath()] = view
	}

	for _, view := range views {
		layouts := view.getApplicableLayouts()
		var layoutViews []*View
		for _, layout := range layouts {
			layoutViews = append(layoutViews, views[layout.RelativePath()])
		}
		//view.applicableLayouts = nil
		view.ApplicableLayoutViews = layoutViews
	}

	return &ViewManager{
		views: views,
	}
}

//ViewByRelPath returns a view by the relative path
func (v *ViewManager) ViewByRelPath(path string) *View {
	view, _ := v.views[path]
	return view
}

//AllViews returns all views
func (v *ViewManager) AllViews() []*View {
	var views []*View
	for _, view := range v.views {
		views = append(views, view)
	}
	return views
}
