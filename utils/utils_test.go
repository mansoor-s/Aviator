package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPathPascalCase(t *testing.T) {
	assert.Equal(t, "ViewsLayout", PathPascalCase("./views/__layout.svelte"))
	assert.Equal(t, "ViewsLayout", PathPascalCase("./Views/__Layout.svelte"))
	assert.Equal(t, "DucksCatsDogsWidget", PathPascalCase("./Ducks/Cats/dogs/widget.svelte"))
	assert.Equal(t, "DucksCatsDogsDogsWidget", PathPascalCase("./Ducks-Cats/dogs/dogs_widget.css"))
	assert.Equal(t, "DucksCatsDogsDogsBirds", PathPascalCase("./Ducks-Cats/dogs/dogsBirds.js"))
}

//tests adapted from https://github.com/iancoleman/strcase/blob/master/camel_test.go
func TestPascalCase(t *testing.T) {
	cases := [][]string{
		{"test_case", "TestCase"},
		{"test.case", "TestCase"},
		{"test", "Test"},
		{"TestCase", "TestCase"},
		{" test  case ", "TestCase"},
		{"", ""},
		{"many_many_words", "ManyManyWords"},
		{"AnyKind of_string", "AnyKindOfString"},
		{"odd-fix", "OddFix"},
		{"numbers2And55with000", "Numbers2And55with000"},
		{"ID", "ID"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := PascalCase(in)
		assert.Equal(t, out, result)
	}
}
