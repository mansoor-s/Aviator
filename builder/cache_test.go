package builder

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheItem_Persistence(t *testing.T) {
	cacheDir := t.TempDir()
	testPath := "/views/catalog/cars.svelte"
	testContent := `function(){console.log("my content is cool")}()`
	item := newCacheItem(cacheDir, testPath, &testContent)

	testDependentPath := "/views/catalog/cats.svelte"
	dependentContent := ""
	testDependent := newCacheItem(
		cacheDir,
		testDependentPath,
		&dependentContent,
	)

	item.AddDependent(testDependent)

	err := item.PersistToFS()
	assert.NoError(t, err)

	files, err := os.ReadDir(cacheDir)
	assert.NoError(t, err)

	assert.Len(t, files, 2)

	expectedCacheFileName := filepath.Join(cacheDir, item.cachedContentHash+".cache")
	assert.FileExists(t, expectedCacheFileName)

	expectedMetadataFileName := filepath.Join(cacheDir, item.cachedContentHash+".metadata")
	assert.FileExists(t, expectedMetadataFileName)

	metadataContent, err := os.ReadFile(expectedMetadataFileName)
	assert.NoError(t, err)

	metadataContentStr := string(metadataContent)
	format := `{"Path":"%s","Dependents":["%s"],"PathContentHash":""}`
	expectedMetadata := fmt.Sprintf(format, testPath, testDependentPath)
	assert.Equal(t, expectedMetadata, metadataContentStr)

	cacheContent, err := os.ReadFile(expectedCacheFileName)
	assert.NoError(t, err)

	assert.Equal(t, testContent, string(cacheContent))
}

func TestCacheItem_Invalidate(t *testing.T) {
	cacheDir := t.TempDir()
	testPath := "/views/catalog/cars.svelte"
	testContent := `function(){console.log("my content is cool")}()`
	item := newCacheItem(cacheDir, testPath, &testContent)

	testDependentPath := "/views/catalog/cats.svelte"
	dependentContent := ""
	testDependent := newCacheItem(
		cacheDir,
		testDependentPath,
		&dependentContent,
	)

	item.AddDependent(testDependent)

	err := item.PersistToFS()
	assert.NoError(t, err)
	err = testDependent.PersistToFS()
	assert.NoError(t, err)

	files, err := os.ReadDir(cacheDir)
	assert.NoError(t, err)

	assert.Len(t, files, 4)

	err = item.Invalidate()
	assert.NoError(t, err)

	filesSecondCheck, err := os.ReadDir(cacheDir)
	assert.NoError(t, err)

	assert.Len(t, filesSecondCheck, 0)

	assert.Equal(t, item.markedForDeletion, true)
	assert.Equal(t, testDependent.markedForDeletion, true)
}

func TestCacheManager(t *testing.T) {
	cacheDir := t.TempDir()
	_, err := newCacheManager(CacheTypeSSR, cacheDir)
	assert.NoError(t, err)

	assert.DirExists(t, filepath.Join(cacheDir, "ssr"))
	assert.NoDirExists(t, filepath.Join(cacheDir, "browser"))

	_, err = newCacheManager(CacheTypeBrowser, cacheDir)
	assert.NoError(t, err)

	assert.DirExists(t, filepath.Join(cacheDir, "ssr"))
	assert.DirExists(t, filepath.Join(cacheDir, "browser"))
}

func TestCacheManager_DependsOn(t *testing.T) {
	cacheDir := t.TempDir()
	testCacheManager, err := newCacheManager(CacheTypeSSR, cacheDir)
	assert.NoError(t, err)

	testPathA := "/views/catalog/cats.svelte"
	testContentA := "foobar"
	testCacheManager.AddCache(testPathA, &testContentA)

	testPathB := "/views/catalog/dogs.svelte"
	//testContentB := "1233445"

	err = testCacheManager.DependsOn(testPathA, testPathB)
	assert.NoError(t, err)

	/*
		//verify the contents are being added to cacheItemDependents correctly
		assert.Len(t, testCacheManager.caches, 1)
		assert.Len(t, testCacheManager.cacheItemDependents, 1)
		assert.Contains(t, testCacheManager.cacheItemDependents, testPathB)
		assert.Len(t, testCacheManager.cacheItemDependents[testPathB], 1)
		assert.Equal(t, testPathA, testCacheManager.cacheItemDependents[testPathB][0].path)

		//verify relationship is being moved to a newly created cache
		//item from cacheItemDependents
		testCacheManager.AddCache(testPathB, &testContentB)
		assert.Len(t, testCacheManager.caches, 2)
		assert.Len(t, testCacheManager.cacheItemDependents, 0)
		assert.Len(t, testCacheManager.caches[testPathB].dependents, 1)
		assert.Contains(t, testCacheManager.caches[testPathB].dependents, testPathA)

	*/
}
