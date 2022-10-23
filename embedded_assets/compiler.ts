import { compile as compileSvelte } from "svelte/compiler"


type Input = {
    code: string
    path: string
    target: "ssr" | "dom"
    dev: boolean
    css: boolean
    enableSourcemap: boolean
    isHydratable: boolean
}

// Capitalized for Go
type Output =
    | {
    JSCode: any
    CSSCode: any
    JSSourceMap: string
    CSSSourceMap: string
}
    | {
    Error: {
        Path: string
        Name: string
        Message: string
        Stack?: string
    }
}

// Compile svelte code

export function compile(input: Input): string {
    const { code, path, target, dev, css, enableSourcemap, isHydratable } = input
    const svelte = compileSvelte(code, {
        filename: path,
        generate: target,
        hydratable: isHydratable,
        format: "esm",
        dev: dev,
        css: css,
        enableSourcemap: enableSourcemap,
    })

    const jsSourceMap = enableSourcemap === true ? svelte.js.map.toUrl() : ""
    const cssSourceMap = enableSourcemap === true ? svelte.css.map.toUrl() : ""

    return JSON.stringify({
        CSSCode: svelte.css.code,
        JSCode: svelte.js.code,
        CSSSourceMap: cssSourceMap,
        JSSourceMap: jsSourceMap,
    } as Output)
}