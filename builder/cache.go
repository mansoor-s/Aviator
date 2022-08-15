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
	"strings"
	"sync"
)

/*
#####View Cache

cacheManager manages cached assets needed by SSRBuilder and BrowserBuilder.
1 instance is created for SSR and 1 is created for Browser builds

It maintains a dependency graph of .svelte files in the project

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

	cacheDir          string
	content           *string
	cachedContentHash string

	//pathContentHash is the hash of the actual file at path, not the compiled content
	pathContentHash string

	//path refers to the original absolute path of the file we're holding a cache for
	path string

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
	//PathContentHash is the hash of the actual file at path
	PathContentHash string
}

func newEmptyCacheItem(cacheFilePath, metadataFilePath string) *cacheItem {
	c := &cacheItem{
		dependents:       map[string]*cacheItem{},
		cacheFilePath:    cacheFilePath,
		metadataFilePath: metadataFilePath,
	}

	return c
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
	c.cachedContentHash = hex.EncodeToString(h.Sum(nil))[:20]

	c.cacheFilePath = filepath.Join(c.cacheDir, c.cachedContentHash+".cache")
	c.metadataFilePath = filepath.Join(c.cacheDir, c.cachedContentHash+".metadata")
	c.pathContentHash = c.pathFileHash()

	return c
}

//IsValid checks to see if cache is not stale by re-hashing the contents
//of the underlying file
func (c *cacheItem) IsValid() bool {
	return c.pathFileHash() == c.pathContentHash
}

func (c *cacheItem) pathFileHash() string {
	fileContent, err := os.ReadFile(c.path)
	//silently return on error if the file is a "virtual" file
	if err != nil {
		return ""
	}

	h := sha1.New()
	h.Write(fileContent)

	return hex.EncodeToString(h.Sum(nil))
}

func (c *cacheItem) readMetadataFile() error {
	fileContent, err := os.ReadFile(c.metadataFilePath)
	if err != nil {
		return err
	}

	metadata := &cacheItemMetadata{}

	err = json.Unmarshal(fileContent, metadata)
	if err != nil {
		return err
	}

	c.path = metadata.Path
	c.pathContentHash = c.pathFileHash()

	for _, dependentPath := range metadata.Dependents {
		// for this stage, just create the record. cacheManger will handle adding the
		//correct reference when all caches are read from FS
		c.dependents[dependentPath] = nil
	}

	return nil
}

func (c *cacheItem) readCacheFile() error {
	fileContent, err := os.ReadFile(c.metadataFilePath)
	if err != nil {
		return err
	}

	contentStr := string(fileContent)
	c.content = &contentStr

	return nil
}

func (c *cacheItem) ReadFS() error {
	err := c.readMetadataFile()
	if err != nil {
		return err
	}

	return c.readMetadataFile()

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
		Path:            c.path,
		Dependents:      dependents,
		PathContentHash: c.pathContentHash,
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

	var skipReadingFromCache bool
	//create cache dir if it doesn't exist
	_, err := os.Stat(c.cacheDir)
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(c.cacheDir, os.ModePerm)
		skipReadingFromCache = true
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if !skipReadingFromCache {
		err = c.readCacheDir()
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *cacheManager) readCacheDir() error {
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	//read all cached content
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) != ".metadata" {
			continue
		}

		nameParts := strings.Split(file.Name(), ".")
		if len(nameParts) != 2 {
			continue
		}

		cachePath := filepath.Join(c.cacheDir, nameParts[0])
		metadataPath := filepath.Join(c.cacheDir, file.Name())

		newCache := newEmptyCacheItem(cachePath, metadataPath)
		err := newCache.ReadFS()
		if err != nil {
			return err
		}
		c.caches[newCache.path] = newCache
	}

	//populate dependents for each cache item now that all caches have been read
	for _, cache := range c.caches {
		for dependentPath := range cache.dependents {
			_, ok := c.caches[dependentPath]
			if !ok {
				return fmt.Errorf(
					`unable to create cache dependency tree because cache with path "%s" doesnt' exist'`,
					dependentPath,
				)
			}
			cache.dependents[dependentPath] = c.caches[dependentPath]
		}
	}

	var cachesPathsToRemove []string
	//verify caches are not stale. if they are, invalidate it and its dependent tree
	for _, cache := range c.caches {
		if !cache.IsValid() {
			err := cache.Invalidate()
			if err != nil {
				return err
			}
			cachesPathsToRemove = append(cachesPathsToRemove, cache.path)
		}
	}

	//remove stale caches
	for _, path := range cachesPathsToRemove {
		delete(c.caches, path)
	}

	return nil
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
