package utils

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const caseDelimiters = `[^a-zA-Z0-9]+`

var caseDelimitersRegexp = regexp.MustCompile(caseDelimiters)

func PathPascalCase(path string) string {
	fileExt := filepath.Ext(path)
	if len(fileExt) > 0 {
		path = path[:len(path)-len(fileExt)]
	}

	return PascalCase(path)
}

//PascalCase turns "some randomText" into SomeRandomText PascalCase (aka UpperCamelCase)
// for simplicity, numbers are not considered delimiters
func PascalCase(str string) string {
	pathParts := caseDelimitersRegexp.Split(str, -1)
	finalStr := ""
	for _, part := range pathParts {
		c := cases.Title(language.Und, cases.NoLower)
		finalStr += c.String(part)
	}

	return finalStr
}

//FileExtension returns the file extension i.e: .js .css .svelte
// returns the extension in lowercase
func FileExtension(fileName string) string {
	nameParts := strings.Split(fileName, ".")
	if len(nameParts) == 1 {
		return ""
	}
	return strings.ToLower(nameParts[len(nameParts)-1])
}

//https://stackoverflow.com/a/33451503
func RemoveDirContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func RecursivelyGetAllChildDirs(path string) ([]string, error) {
	var childDirs []string

	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		childPath := filepath.Join(path, dir.Name())
		childDirs = append(childDirs, childPath)

		childsDescendants, err := RecursivelyGetAllChildDirs(childPath)
		if err != nil {
			return nil, err
		}
		childDirs = append(childDirs, childsDescendants...)
	}

	return childDirs, nil
}
