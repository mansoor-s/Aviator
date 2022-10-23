package builder

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type ssrData struct {
	Head string
	Body string

	//this is created during bundling
	BundledCSS string

	//these pare provided by the user
	Lang string
}

func (v *ViewManager) Render(
	_ context.Context,
	viewPath string,
	props interface{},
) (string, error) {
	view := v.ViewByRelPath(viewPath)

	if view == nil {
		return "", fmt.Errorf("view does not exist in path %s", viewPath)
	}

	//TODO: Create a sanitized copy of the props object where
	// string objects are escaped to avoid script injections on the front end
	// Should users be able to bypass escaping using tags?
	jsonValue := "{}"
	if props != nil {
		jsonProps, err := json.Marshal(props)
		if err != nil {
			return "", fmt.Errorf("failed to json serialize props %w", err)
		}
		jsonValue = string(jsonProps)
	}

	expr := fmt.Sprintf(
		"; __aviator__.render(%q, %s, {})",
		view.WrappedUniqueName,
		jsonValue,
	)
	renderOutputStr, err := v.vm.Eval("runtime_renderer", expr)
	if err != nil {
		return renderOutputStr, err
	}

	ssrOutputData := &ssrData{}
	err = json.Unmarshal([]byte(renderOutputStr), ssrOutputData)
	if err != nil {
		return "", err
	}

	ssrOutputData.Head = ssrOutputData.Head + "\n" +
		v.createJSImportTags(view.JSImports)

	_, baseStyleFound := v.staticContent[baseCSSStyleName]
	if baseStyleFound {
		ssrOutputData.Head += v.createCSSImportTag(baseCSSStyleName)
	}

	ssrOutputData.Head +=
		v.createCSSImportTags(view.CSSImports) +
			v.createPropsScriptElem(jsonValue)

	ssrOutputData.Lang = v.htmlLang
	//cssPath := path.Join(a.assetListenPath, a._compiledCSSFileName)
	//ssrOutputData.BundledCSS = "<link href=\"" + cssPath + "\" rel=\"stylesheet\">"

	buf := new(bytes.Buffer)
	err = v.htmlGenerator.Execute(buf, ssrOutputData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (v *ViewManager) GetStaticAsset(name string) (StaticAsset, bool) {
	staticAsset, ok := v.staticContent[name]
	return staticAsset, ok
}

func (v *ViewManager) createPropsScriptElem(props string) string {
	format := "<script id=\"__aviator_props\" type=\"text/template\" defer>%s</script>\n"
	return fmt.Sprintf(format, props)
}

func (v *ViewManager) createJSImportTags(assetImports []string) string {
	output := ""
	format := "<script type=\"module\" src=\"%s\" defer></script>\n"
	for _, rawPath := range assetImports {
		output += fmt.Sprintf(format, filepath.Join(v.staticAssetsRoute, rawPath))
	}

	return output
}
func (v *ViewManager) createCSSImportTags(assetImports []string) string {
	output := ""
	for _, rawPath := range assetImports {
		output += v.createCSSImportTag(rawPath)
	}
	return output
}

func (v *ViewManager) createCSSImportTag(path string) string {
	format := "<link href=\"%s\" rel=\"stylesheet\">\n"
	return fmt.Sprintf(format, filepath.Join(v.staticAssetsRoute, path))

}
