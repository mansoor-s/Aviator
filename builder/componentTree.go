package builder

import (
	"github.com/mansoor-s/aviator/utils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//for every component determine the appropriate __layout file
//for each __layout + component file create an ephemeral entrypoint file
//implication: this will create more ephemeral files than actually used because non-page
//	components will also incorrectly get their own files. This should be okay for MVP

//run esbuild on each of the ephemeral entrypoint files to create a renderable view

/*
Determining the right __layout file:
	Does element use a named layout?			https://kit.svelte.dev/docs/layouts#named-layouts
		Yes:
			Use named layout in component's directory
			if not found, walk up to all the component's parent directories to search for named layout
		No:
			Use __layout in component's directory
			if not found, walk up to all the component's parent directories to search for __layout

	if no __layout files are found, the default layout file with <slot></slot> is used
*/

const npmDir = "node_modules"

type Layout struct {
	Name string

	//Path is absolute
	Path string

	// nil if no parent layout exists
	ParentLayout *Layout

	// TODO: implement layout resets
	//if layout is a reset layout, don't inherit parent layout
	isAResetLayout bool

	// ParentTree represents the directory this layout belongs in
	ParentTree *componentTree

	//used temporarily until the parent layout Path can be resolved
	parentLayoutName string

	rootTree *componentTree
}

func (l *Layout) ApplicableLayouts() []*Layout {
	var ancestors []*Layout
	if l.isAResetLayout {
		return ancestors
	}

	if l.ParentLayout != nil {
		ancestors = append(ancestors, l.ParentLayout)
		ancestors = append(ancestors, l.ParentLayout.ApplicableLayouts()...)
	}

	return ancestors
}

//RelativePath returns the relative Path of the layout to the views directory
func (l *Layout) RelativePath() string {
	relPath, _ := filepath.Rel(l.rootTree.path, l.Path)
	return relPath
}

type Component struct {
	Name string

	//Path is absolute
	Path string

	Layout *Layout

	// ParentTree represents the directory this component belongs in
	ParentTree *componentTree

	//used temporarily until the layout Path can be resolved
	layoutName string

	rootTree *componentTree
}

//RelativePath returns the relative Path of the component to the views directory
func (c *Component) RelativePath() string {
	relPath, _ := filepath.Rel(c.rootTree.path, c.Path)
	return relPath
}

//ApplicableLayouts returns a flattened layout hierarchy of layouts applicable
//to this component. Layout index determines its proximity to the component
//in the hierarchy
func (c *Component) ApplicableLayouts() []*Layout {
	if c.Layout == nil {
		return nil
	}

	layouts := []*Layout{c.Layout}

	layouts = append(layouts, c.Layout.ApplicableLayouts()...)

	return layouts
}

type ComponentTree interface {
	ResolveLayoutByName(name string) *Layout
	Path() string
	GetAllComponents() []*Component
	GetAllLayouts() []*Layout
	GetAllDescendantPaths() []string
}

type componentTree struct {
	//absolute path
	path string

	Components []*Component
	Layouts    map[string]*Layout
	Children   []*componentTree

	Parent *componentTree

	rootTree *componentTree
}

//CreateComponentTree creates a componentTree based on the absolute Path
// it performs a depth-first search through all subdirectories under
// the specified Path
func CreateComponentTree(path string) (*componentTree, error) {
	return createComponentTree(nil, path)

}

func createComponentTree(parentTree *componentTree, path string) (*componentTree, error) {
	tree := &componentTree{
		path:   path,
		Parent: parentTree,
	}

	if parentTree != nil {
		tree.rootTree = tree.Parent.rootTree
	} else {
		tree.rootTree = tree
	}

	// first find all +layouts
	err := tree.findLayouts()
	if err != nil {
		return nil, err
	}

	//resolve +layout parents if any
	tree.resolveLayoutParents()

	// find all component at current Path
	err = tree.findComponents()
	if err != nil {
		return nil, err
	}

	// resolve layouts for components
	tree.resolveComponentLayouts()

	//walk through child directories to find layouts and components
	err = tree.findChildTrees()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

func (c *componentTree) Path() string {
	return c.path
}

// GetAllComponents returns all components at this tree level and child levels
func (c *componentTree) GetAllComponents() []*Component {
	components := c.Components

	for _, childTree := range c.Children {
		components = append(components, childTree.GetAllComponents()...)
	}

	return components
}

// GetAllLayouts returns all layouts at this tree level and child levels
func (c *componentTree) GetAllLayouts() []*Layout {
	var layouts []*Layout

	for _, layout := range c.Layouts {
		layouts = append(layouts, layout)
	}

	for _, childTree := range c.Children {
		layouts = append(layouts, childTree.GetAllLayouts()...)
	}

	return layouts
}

// GetAllDescendantPaths returns all descendant paths starting from this dir level
// No order guarantees
func (c *componentTree) GetAllDescendantPaths() []string {
	paths := []string{
		c.Path(),
	}

	for _, childTree := range c.Children {
		paths = append(paths, childTree.Path())
		paths = append(paths, childTree.GetAllDescendantPaths()...)
	}

	return paths
}

//findChildTrees walks through all child directories and recursively
// creates a componentTree for each
func (c *componentTree) findChildTrees() error {
	var children []*componentTree

	dirs, err := os.ReadDir(c.path)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		//skip node_modules
		if dir.Name() == npmDir {
			continue
		}

		child, err := createComponentTree(c, filepath.Join(c.path, dir.Name()))
		if err != nil {
			return err
		}
		children = append(children, child)
	}

	c.Children = children

	return nil
}

func (c *componentTree) resolveComponentLayouts() {
	for _, component := range c.Components {
		//if component layout is empty string, interpret it as +layout
		layoutName := component.layoutName
		if len(layoutName) == 0 {
			layoutName = "+layout"
		}
		component.Layout = c.ResolveLayoutByName(layoutName)
	}
}

var svelteFileRegexp = regexp.MustCompile(`.*\.svelte$`)
var svelteLayoutRegexp = regexp.MustCompile(`\+layout.*\.svelte$`)

//finds all component files in current tree level (aka directory depth)
func (c *componentTree) findComponents() error {
	files, err := os.ReadDir(c.path)
	if err != nil {
		return err
	}

	var components []*Component

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		//skip layout files
		isLayoutFile := svelteLayoutRegexp.MatchString(file.Name())
		if isLayoutFile {
			continue
		}

		isMatch := svelteFileRegexp.MatchString(file.Name())
		if !isMatch {
			continue
		}

		componentName, layoutName := getComponentWithLayoutName(file.Name())
		components = append(components, &Component{
			Name:       utils.PascalCase(componentName),
			Path:       filepath.Join(c.path, file.Name()),
			layoutName: layoutName,
			ParentTree: c,
			rootTree:   c.rootTree,
		})
	}
	c.Components = components

	return nil
}

//finds all +layout files in current tree level (aka directory depth)
func (c *componentTree) findLayouts() error {
	files, err := os.ReadDir(c.path)
	if err != nil {
		return err
	}

	layouts := make(map[string]*Layout)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		isMatch := svelteLayoutRegexp.MatchString(file.Name())
		if !isMatch {
			continue
		}

		layoutName, layoutParent := getLayoutInfo(file.Name())
		layouts[layoutName] = &Layout{
			Name:             layoutName,
			Path:             filepath.Join(c.path, file.Name()),
			parentLayoutName: layoutParent,
			ParentTree:       c,
			rootTree:         c.rootTree,
		}
	}

	c.Layouts = layouts

	return nil
}

//getLayoutInfo returns the layout name and parent layout name if it exists
// will return an empty string if a parent layout is not in the name
func getLayoutInfo(path string) (string, string) {
	fileName := strings.Split(path, ".")[0]

	nameParts := strings.Split(fileName, "-")
	if len(nameParts) == 1 {
		return getLayoutWithParentName(fileName)
	}

	return getLayoutWithParentName(nameParts[1])
}

func getLayoutWithParentName(name string) (string, string) {
	nameParts := strings.Split(name, "@")
	if len(nameParts) == 1 {
		return name, ""
	}

	return nameParts[0], nameParts[1]
}

//getComponentWithLayoutName returns the component file name (without the layout part of the name)
//along with the layout name if one exists in its name
func getComponentWithLayoutName(path string) (string, string) {
	fileName := strings.Split(path, ".")[0]

	nameParts := strings.Split(fileName, "@")
	if len(nameParts) == 1 {
		return fileName, ""
	}

	return nameParts[0], nameParts[1]
}

//ResolveLayoutByName recursively finds the named layout.
// first it will search in the current component tree level
// if it can't find it, it will walk up to all the ancestor trees
// returns nil if a layout is not found
func (c *componentTree) ResolveLayoutByName(name string) *Layout {
	layout, ok := c.Layouts[name]
	if ok {
		return layout
	}

	if c.Parent != nil {
		return c.Parent.ResolveLayoutByName(name)
	}

	return nil
}

func (c *componentTree) resolveLayoutParents() {
	for _, layout := range c.Layouts {
		if len(layout.parentLayoutName) == 0 {
			continue
		}
		layout.ParentLayout = c.ResolveLayoutByName(layout.parentLayoutName)
	}
}
