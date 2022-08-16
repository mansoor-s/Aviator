package builder

import (
	"fmt"
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

	Components map[string]*Component //[]*Component
	Layouts    map[string]*Layout
	Children   map[string]*componentTree //[]*componentTree

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
		path:       path,
		Parent:     parentTree,
		Components: make(map[string]*Component),
		Layouts:    make(map[string]*Layout),
		Children:   make(map[string]*componentTree),
	}

	if parentTree != nil {
		tree.rootTree = tree.Parent.rootTree
	} else {
		tree.rootTree = tree
	}

	err := tree.ReScan()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

//ReScan forces the componentTree to rescan it's children and child components and layouts
//starting only at the directory depth associated with this tree
//ReScan will NOT walk down to child trees
func (c *componentTree) ReScan() error {
	// first find all +layouts
	err := c.findLayouts()
	if err != nil {
		return err
	}

	//resolve +layout parents if any
	c.resolveLayoutParents()

	// find all component at current Path
	err = c.findComponents()
	if err != nil {
		return err
	}

	// resolve layouts for components
	c.resolveComponentLayouts()

	//walk through child directories to find layouts and components
	err = c.findChildTrees()
	if err != nil {
		return err
	}

	return nil
}

func (c *componentTree) Path() string {
	return c.path
}

// GetAllComponents returns all components at this tree level and child levels
func (c *componentTree) GetAllComponents() []*Component {
	var components []*Component

	for _, component := range c.Components {
		components = append(components, component)
	}

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

// GetAllDescendentTrees returns all descendent trees starting from this tree level
// No order guarantees
func (c *componentTree) GetAllDescendentTrees() map[string]*componentTree {
	descendents := map[string]*componentTree{
		c.Path(): c,
	}

	for _, childTree := range c.Children {
		descendents[childTree.path] = childTree
		childDescendents := childTree.GetAllDescendentTrees()
		for _, tree := range childDescendents {
			descendents[tree.path] = tree
		}
	}

	return descendents
}

//findChildTrees walks through all child directories and recursively
// creates a componentTree for each if one doesn't exist
func (c *componentTree) findChildTrees() error {
	dirs, err := os.ReadDir(c.path)
	if err != nil {
		return err
	}

	childDirsInPath := map[string]struct{}{}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		//skip node_modules
		if dir.Name() == npmDir {
			continue
		}

		childPath := filepath.Join(c.path, dir.Name())
		childDirsInPath[childPath] = struct{}{}

		//skip if child already exists
		_, ok := c.Children[childPath]
		if ok {
			continue
		}

		child, err := createComponentTree(c, childPath)
		if err != nil {
			return err
		}
		c.Children[childPath] = child
	}

	//remove child trees that have been removed on the FS
	for _, child := range c.Children {
		_, ok := childDirsInPath[child.path]
		if !ok {
			delete(c.Children, child.path)
		}
	}

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

	var componentsInDir []string

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
		//skip if it was already added
		_, ok := c.Components[componentName]
		if ok {
			continue
		}
		componentsInDir = append(componentsInDir, componentName)
		c.Components[componentName] = &Component{
			Name:       utils.PascalCase(componentName),
			Path:       filepath.Join(c.path, file.Name()),
			layoutName: layoutName,
			ParentTree: c,
			rootTree:   c.rootTree,
		}
	}

	//remove stale components that are no longer in the FS
	for _, componentName := range componentsInDir {
		delete(c.Layouts, componentName)
	}

	return nil
}

//finds all +layout files in current tree level (aka directory depth)
func (c *componentTree) findLayouts() error {
	files, err := os.ReadDir(c.path)
	if err != nil {
		return err
	}

	var layoutsInDir []string

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		isMatch := svelteLayoutRegexp.MatchString(file.Name())
		if !isMatch {
			continue
		}

		layoutName, layoutParent := getLayoutInfo(file.Name())
		//if layout already exists, skip it
		_, ok := c.Layouts[layoutName]
		if ok {
			continue
		}

		layoutsInDir = append(layoutsInDir, layoutName)
		c.Layouts[layoutName] = &Layout{
			Name:             layoutName,
			Path:             filepath.Join(c.path, file.Name()),
			parentLayoutName: layoutParent,
			ParentTree:       c,
			rootTree:         c.rootTree,
		}
	}

	//remove stale layouts that are no longer in the FS
	for _, layoutName := range layoutsInDir {
		delete(c.Layouts, layoutName)
	}

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

//RescanDir rescans the path to add / remove files and directories
// if path is a file, it will just look at the directory portion of the path
func (c *componentTree) RescanDir(path string) error {
	allTrees := c.GetAllDescendentTrees()

	parentDir := filepath.Dir(path)
	parentTree, ok := allTrees[parentDir]
	if !ok {
		return fmt.Errorf(
			`unable to add dir at path "%s" because parent directory was not present in component tree'`,
			parentDir,
		)
	}

	return parentTree.ReScan()
}
