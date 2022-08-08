import { compile as compileSvelte } from "svelte/compiler"


type Input = {
    code: string
    path: string
    target: "ssr" | "dom"
    dev: boolean
    css: boolean
    enableSourcemap: boolean
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
    const { code, path, target, dev, css, enableSourcemap } = input
    const svelte = compileSvelte(code, {
        filename: path,
        generate: target,
        hydratable: true,
        format: "esm",
        dev: dev,
        css: css,
        enableSourcemap: enableSourcemap,
    })

    return JSON.stringify({
        CSSCode: svelte.css.code,
        JSCode: svelte.js.code,
        CSSSourceMap: "", //svelte.css.map.toUrl(),
        JSSourceMap: svelte.js.map.toUrl(),
    } as Output)
}
