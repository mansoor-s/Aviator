package builder

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mansoor-s/aviator/js"
	"github.com/mansoor-s/aviator/utils"
	"github.com/mansoor-s/aviator/watcher"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

/*


######ComponentTree
ComponentTree finds all views and constructs either a View or Layout object representing the file
It also builds relationships between Layouts to Layouts and Views to Layouts

It can also return the correct parent layout that applies to the view or layout

New methods needed which operate on both Layouts and Components:

AddSvelteFile(Path)
RemoveSvelteFile(Path)

A rename event is just the above two since things like applicable layout files could have changed.



#####View manager
View manager holds a record of all views

It triggers views scan by using ComponentTree
It triggers SSR and Browser builds


It renders the view when requested

*/

const eventBatchTime = 500 * time.Millisecond

type ViewManager struct {
	viewsDir  string
	cacheDir  string
	isDevMode bool
	tree      *componentTree
	vm        js.VM

	//ssrCacheManager     *cacheManager
	//browserCacheManager *cacheManager
	watcher *watcher.Batcher
	views   map[string]*View

	ssrCache     *cacheManager
	browserCache *cacheManager

	browserBuilder *BrowserBuilder
	ssrBuilder     *SSRBuilder
	logger         utils.Logger

	sync.Mutex
}

func NewViewManager(
	logger utils.Logger,
	vm js.VM,
	tree ComponentTree,
	isDevMode bool,
	cacheDir string,
	viewsDir string,
) (*ViewManager, error) {
	viewWatcher, err := watcher.New(eventBatchTime)
	if err != nil {
		return nil, err
	}

	ssrCache, err := newCacheManager(CacheTypeSSR, cacheDir)
	if err != nil {
		return nil, err
	}

	browserCache, err := newCacheManager(CacheTypeBrowser, cacheDir)
	if err != nil {
		return nil, err
	}

	ssrBuilder := NewSSRBuilder(logger, vm, ssrCache, viewsDir)
	browserBuilder := NewBrowserBuilder(logger, vm, browserCache, viewsDir)
	v := &ViewManager{
		vm:             vm,
		logger:         logger,
		watcher:        viewWatcher,
		tree:           tree.(*componentTree),
		isDevMode:      isDevMode,
		browserBuilder: browserBuilder,
		ssrBuilder:     ssrBuilder,
		ssrCache:       ssrCache,
		browserCache:   browserCache,
		viewsDir:       viewsDir,
	}

	v.refreshViews()

	err = v.Build()

	return v, err
}

func (v *ViewManager) Build() error {
	allViews := v.AllViews()

	err := v.browserBuilder.BuildDev(allViews)
	if err != nil {
		v.logger.Error("error building SSR build: " + err.Error())
		return err
	}

	err = v.browserCache.Persist()
	if err != nil {
		v.logger.Error("error persisting Browser cache: " + err.Error())
		return err
	}

	ssrBuild, err := v.ssrBuilder.DevBuild(allViews)
	if err != nil {
		v.logger.Error("error building Browser build: " + err.Error())
		return err
	}

	err = v.ssrCache.Persist()
	if err != nil {
		v.logger.Error("error persisting SSR cache: " + err.Error())
		return err
	}

	_, err = v.vm.Eval(
		"aviator_ssr_router.js",
		string(ssrBuild.JS),
	)

	return err
}

func (v *ViewManager) Render(viewPath string) error {
	view := v.ViewByRelPath(viewPath)
	if view == nil {
		return fmt.Errorf(`view in path "%s" not found`, viewPath)
	}

	return nil
}

func (v *ViewManager) refreshViews() {
	v.views = map[string]*View{}

	for _, component := range v.tree.GetAllComponents() {
		view := newViewFromComponent(component)
		view.applicableLayouts = component.ApplicableLayouts()
		v.views[component.RelativePath()] = view
	}

	for _, layout := range v.tree.GetAllLayouts() {
		view := newViewFromLayout(layout)
		view.applicableLayouts = layout.ApplicableLayouts()
		v.views[layout.RelativePath()] = view
	}

	for _, view := range v.views {
		layouts := view.getApplicableLayouts()
		var layoutViews []*View
		for _, layout := range layouts {
			layoutViews = append(layoutViews, v.views[layout.RelativePath()])
		}

		view.ApplicableLayoutViews = layoutViews
	}
}

//ViewByRelPath returns a view by the relative Path
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

//StartWatch starts watching views directory for changes
func (v *ViewManager) StartWatch() error {
	//fsnotify doesn't currently support watching a directory recursively, so we must
	//manually watch each child directory here
	for _, dirPath := range v.tree.GetAllDescendantPaths() {
		v.logger.Info("Starting watch on dir: " + dirPath)
		err := v.watcher.Add(dirPath)
		if err != nil {
			return err
		}
	}

	go func() {
		for {
			select {
			case events, _ := <-v.watcher.Events:
				err := v.handleEvents(events)
				if err != nil {
					v.logger.Error(
						fmt.Errorf(`error while handling view files changes: %w`,
							err).Error(),
					)
				}
			case err, ok := <-v.watcher.Errors():
				if !ok {
					return
				}
				v.logger.Error(
					fmt.Errorf(`error while watching view files: %w`, err).Error(),
				)
			}
		}
	}()

	return nil
}

func (v *ViewManager) handleEvents(events []fsnotify.Event) error {
	v.Lock()
	defer v.Unlock()

	numHandledEvents := 0
	for _, e := range events {
		v.logger.Info("Handling event")
		//skip events on editor created temp files
		if isTempFile(e.Name) || e.Name == "" {
			v.logger.Info("Skipping temp file: " + e.Name)
			continue
		}

		numHandledEvents++

		if e.Op&fsnotify.Create == fsnotify.Create {
			err := v.handleCreateEvent(e)
			if err != nil {
				return err
			}
		}

		if e.Op&fsnotify.Remove == fsnotify.Remove {
			err := v.handleRemoveEvent(e)
			if err != nil {
				return err
			}
		}

		//invalidate cache
		if e.Op&fsnotify.Write == fsnotify.Write {
			err := v.handleWriteEvent(e)
			if err != nil {
				return err
			}
		}

		//invalidate cache
		if e.Op&fsnotify.Rename == fsnotify.Rename {
			err := v.handleRenameEvent(e)
			if err != nil {
				return err
			}
		}
	}

	if numHandledEvents > 0 {
		v.refreshViews()
		err := v.Build()
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *ViewManager) handleRenameEvent(e fsnotify.Event) error {
	err := v.ssrCache.Invalidate(e.Name)
	if err != nil {
		return err
	}

	err = v.browserCache.Invalidate(e.Name)
	if err != nil {
		return err
	}

	rescanPath := filepath.Base(e.Name)

	//rescan the parent dir for both file and dir removal
	return v.tree.RescanDir(rescanPath)
}

func (v *ViewManager) handleWriteEvent(e fsnotify.Event) error {
	v.logger.Info("File updated: " + e.Name)
	err := v.ssrCache.Invalidate(e.Name)
	if err != nil {
		return err
	}

	return v.browserCache.Invalidate(e.Name)
}

func (v *ViewManager) handleRemoveEvent(e fsnotify.Event) error {
	err := v.ssrCache.Invalidate(e.Name)
	if err != nil {
		return err
	}

	err = v.browserCache.Invalidate(e.Name)
	if err != nil {
		return err
	}

	rescanPath := filepath.Base(e.Name)

	//rescan the parent dir for both file and dir removal
	return v.tree.RescanDir(rescanPath)
}

func (v *ViewManager) handleCreateEvent(e fsnotify.Event) error {
	fileInfo, err := os.Stat(e.Name)
	if err != nil {
		return err
	}

	rescanPath := filepath.Base(e.Name)

	if fileInfo.IsDir() {
		// recursively add new directories to watch list
		// When mkdir -p is used, only the top directory triggers an event (at least on OSX)
		dirs, err := utils.RecursivelyGetAllChildDirs(e.Name)
		if err != nil {
			return err
		}

		for _, dir := range dirs {
			err := v.watcher.Add(dir)
			if err != nil {
				return err
			}
		}
	}

	//rescan the parent dir for both file and dir creation
	return v.tree.RescanDir(rescanPath)
}

// from Hugo
// https://github.com/gohugoio/hugo/blob/cbc35c48d252a1b44e4c30e26cfba2ff462a1f96/commands/hugo.go#L1039
func isTempFile(name string) bool {
	ext := filepath.Ext(name)
	baseName := filepath.Base(name)
	isTemp := strings.HasSuffix(ext, "~") ||
		(ext == ".swp") || // vim
		(ext == ".swx") || // vim
		(ext == ".tmp") || // generic temp file
		(ext == ".DS_Store") || // OSX Thumbnail
		baseName == "4913" || // vim
		strings.HasPrefix(ext, ".goutputstream") || // gnome
		strings.HasSuffix(ext, "jb_old___") || // intelliJ
		strings.HasSuffix(ext, "jb_tmp___") || // intelliJ
		strings.HasSuffix(ext, "jb_bak___") || // intelliJ
		strings.HasPrefix(ext, ".sb-") || // byword
		strings.HasPrefix(baseName, ".#") || // emacs
		strings.HasPrefix(baseName, "#") // emacs

	return isTemp
}
