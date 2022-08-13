package builder

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

/*
#####View Cache

cacheManager manages cached assets needed by SSRBuilder and BrowserBuilder.
1 instance is created for SSR and 1 is created for Browser builds

It maintains a dependency graph of .svelte files in both the project and node_packages

It will Invalidate all caches for dependants of a changed asset

It acts a "pass-through" to return actual FS content when file isn't cached

It will load cached assets from FS on application start in dev mode

it will persist cached content to FS when a new build is done


FS cache will create two directories, 1 for SSR and 1 for Browser

FS cache will create two files per .svelte file:
	1: svelte_SHA256
	2: metadata_SHA256
SHA256.cache is the cached compiled .svelte file
SHA256.metadata is a JSON file with contents:
{
	"Path": "", //full Path
	"dependents": ["file Path"]
}

API:
AddCache(importPath, content)
AddImport(importerPath, importedPath)
*/

const (
	CacheTypeSSR = iota
	CacheTypeBrowser
)

type cacheItem struct {
	cacheType int

	cacheDir string
	content  *string

	//path refers to the original absolute path of the file we're holding a cache for
	path string
	hash string

	cacheFilePath    string
	metadataFilePath string

	pendingCacheWrite    bool
	pendingMetadataWrite bool

	//indicate if cache data should be deleted if it isn't being dependent on by anything
	markedForDeletion bool

	dependents map[string]*cacheItem
}

type cacheItemMetadata struct {
	Path       string
	Dependents []string
}

func newCacheItem(cacheDir, path string, content *string) *cacheItem {
	c := &cacheItem{
		cacheDir:             cacheDir,
		path:                 path,
		content:              content,
		dependents:           map[string]*cacheItem{},
		pendingCacheWrite:    true,
		pendingMetadataWrite: true,
	}

	h := sha1.New()
	io.WriteString(h, *content)
	c.hash = hex.EncodeToString(h.Sum(nil))[:20]

	c.cacheFilePath = filepath.Join(c.cacheDir, c.hash+".svelte")
	c.metadataFilePath = filepath.Join(c.cacheDir, c.hash+".metadata")

	return c
}

func (c *cacheItem) writeCacheFile() error {
	cacheF, err := os.Create(c.cacheFilePath)
	if err != nil {
		return err
	}
	defer cacheF.Close()

	_, err = cacheF.WriteString(*c.content)
	if err != nil {
		return err
	}

	c.pendingCacheWrite = false
	return nil
}

func (c *cacheItem) writeMetadataFile() error {
	metaF, err := os.Create(c.metadataFilePath)
	if err != nil {
		return err
	}
	defer metaF.Close()

	var dependents []string
	for _, dep := range c.dependents {
		dependents = append(dependents, dep.path)
	}

	metadata := cacheItemMetadata{
		Path:       c.path,
		Dependents: dependents,
	}
	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = metaF.Write(metadataJson)
	if err != nil {
		return err
	}

	c.pendingMetadataWrite = false
	return nil
}

func (c *cacheItem) PersistToFS() error {
	if c.pendingCacheWrite {
		err := c.writeCacheFile()
		if err != nil {
			return err
		}
	}

	if c.pendingMetadataWrite {
		err := c.writeMetadataFile()
		if err != nil {
			return err
		}
	}

	return nil
}

//Invalidate deletes FS cache and notifies cacheItems that depend
//on this item to Invalidate themselves
func (c *cacheItem) Invalidate() error {
	c.markedForDeletion = true

	err := os.Remove(c.cacheFilePath)
	if err != nil {
		return err
	}
	err = os.Remove(c.metadataFilePath)
	if err != nil {
		return err
	}

	for _, dependent := range c.dependents {
		err = dependent.Invalidate()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *cacheItem) RemoveDependent(dependant *cacheItem) {
	delete(c.dependents, dependant.path)

	if len(c.dependents) == 0 {
		c.markedForDeletion = true
	}
}

func (c *cacheItem) AddDependent(dependant *cacheItem) {
	c.dependents[dependant.path] = dependant
	c.markedForDeletion = false
}

type cacheManager struct {
	cacheType int
	cacheDir  string

	caches map[string]*cacheItem

	//temporary dependant storage for when the cache item doesn't exist yet,
	//but it is being referenced by another cacheItem
	//when the cacheItem is created, the dependants are moved there and cleared from here
	cacheItemDependent map[string][]*cacheItem

	sync.Mutex
}

func newCacheManager(cacheType int, cacheDir string) (*cacheManager, error) {
	cacheTypeStr := "ssr"
	if cacheType == CacheTypeBrowser {
		cacheTypeStr = "browser"
	}

	c := &cacheManager{
		cacheType:          cacheType,
		cacheDir:           filepath.Join(cacheDir, cacheTypeStr),
		caches:             map[string]*cacheItem{},
		cacheItemDependent: map[string][]*cacheItem{},
	}

	//create cache dir if it doesn't exist
	_, err := os.Stat(c.cacheDir)
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(c.cacheDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return c, nil
}

//GetContent returns the cached content if it exists, else it returns nil
func (c *cacheManager) GetContent(path string) *string {
	c.Lock()
	defer c.Unlock()

	cache, ok := c.caches[path]
	if !ok {
		return nil
	}

	return cache.content
}

func (c *cacheManager) DependsOn(pathA, pathB string) error {
	c.Lock()
	defer c.Unlock()

	//assume pathA cache exists, otherwise we wouldn't be resolving imports
	cacheA, ok := c.caches[pathA]
	if !ok {
		return fmt.Errorf(`expected cache for "%s" to exist`, pathA)
	}

	cacheB, ok := c.caches[pathB]
	if ok {
		cacheB.AddDependent(cacheA)
	} else {
		c.cacheItemDependent[pathB] = append(c.cacheItemDependent[pathB], cacheA)
	}

	return nil
}

//AddCache creates a cache object for the file being cached
func (c *cacheManager) AddCache(path string, content *string) {
	c.Lock()
	defer c.Unlock()

	cache := newCacheItem(c.cacheDir, path, content)

	dependents, ok := c.cacheItemDependent[path]
	if ok {
		for _, dependent := range dependents {
			cache.AddDependent(dependent)
		}

		delete(c.cacheItemDependent, path)
	}

	//overwrite Path if it already exists
	c.caches[path] = cache
}

func (c *cacheManager) Invalidate(path string) error {
	c.Lock()
	defer c.Unlock()

	cache, ok := c.caches[path]
	if !ok {
		return fmt.Errorf(`expected cache for "%s" to exist`, path)
	}

	err := cache.Invalidate()
	if err != nil {
		return err
	}

	delete(c.caches, path)

	return nil
}
