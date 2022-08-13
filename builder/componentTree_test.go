package builder

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestGetLayoutName(t *testing.T) {
	expectedName1 := "+layout"
	name1, parent1 := getLayoutInfo("+layout.svelte")
	assert.Equal(t, expectedName1, name1)
	assert.Equal(t, parent1, "")

	expectedName2 := "foobar"
	name2, parent2 := getLayoutInfo("+layout-foobar.svelte")
	assert.Equal(t, expectedName2, name2)
	assert.Equal(t, parent2, "")

	expectedName3 := "foobar"
	expectedParent1 := "cat"
	name3, parent3 := getLayoutInfo("+layout-foobar@cat.svelte")
	assert.Equal(t, expectedName3, name3)
	assert.Equal(t, expectedParent1, parent3)

	expectedName4 := "+layout"
	expectedParent2 := "cat"
	name4, parent4 := getLayoutInfo("+layout@cat.svelte")
	assert.Equal(t, expectedName4, name4)
	assert.Equal(t, expectedParent2, parent4)
}

func TestGetComponentWithLayoutName(t *testing.T) {
	expectedComponentName1 := "index"
	expectedLayout1 := ""
	compName1, layoutName1 := getComponentWithLayoutName("index.svelte")
	assert.Equal(t, expectedComponentName1, compName1)
	assert.Equal(t, expectedLayout1, layoutName1)

	expectedComponentName2 := "index-page"
	expectedLayout2 := ""
	compName2, layoutName2 := getComponentWithLayoutName("index-page.svelte")
	assert.Equal(t, expectedComponentName2, compName2)
	assert.Equal(t, expectedLayout2, layoutName2)

	expectedComponentName3 := "index-page"
	expectedLayout3 := "foo"
	compName3, layoutName3 := getComponentWithLayoutName("index-page@foo.svelte")
	assert.Equal(t, expectedComponentName3, compName3)
	assert.Equal(t, expectedLayout3, layoutName3)
}

//TODO: add tests here. Confirmed via debugger it is working as expected
func TestCreateComponentTree(t *testing.T) {
	absPath, err := filepath.Abs("./test_data/views")
	assert.NoErrorf(t, err, "finding abs Path should not return error")

	tree, err := CreateComponentTree(absPath)
	assert.NoError(t, err)
	assert.NotNil(t, tree)
}

func TestComponent_RelativePath(t *testing.T) {
	absPath, err := filepath.Abs("./test_data/views")
	assert.NoErrorf(t, err, "finding abs Path should not return error")

	tree, err := CreateComponentTree(absPath)
	assert.NoError(t, err)

	allComponents := tree.GetAllComponents()

	testComponent1RelPath := "index.svelte"
	testComponent1Path := filepath.Join(absPath, testComponent1RelPath)
	var testComponent1 *Component

	testComponent2RelPath := "users/get-users.svelte"
	testComponent2Path := filepath.Join(absPath, testComponent2RelPath)
	var testComponent2 *Component

	testComponent3RelPath := "users/users-subdirectory/users-listing@users.svelte"
	testComponent3Path := filepath.Join(absPath, testComponent3RelPath)
	var testComponent3 *Component

	//find our test components
	for _, component := range allComponents {
		if component.Path == testComponent1Path {
			testComponent1 = component
		}
		if component.Path == testComponent2Path {
			testComponent2 = component
		}
		if component.Path == testComponent3Path {
			testComponent3 = component
		}
	}

	assert.NotNil(t, testComponent1)
	assert.NotNil(t, testComponent2)
	assert.NotNil(t, testComponent3)

	assert.Equal(t, testComponent1RelPath, testComponent1.RelativePath())
	assert.Equal(t, testComponent2RelPath, testComponent2.RelativePath())
	assert.Equal(t, testComponent3RelPath, testComponent3.RelativePath())
}
