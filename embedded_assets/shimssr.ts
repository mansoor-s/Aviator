/**
 * Shim for getting the svelte compiler to run in a V8 isolate.
 */

// URL shim for the browser
// TODO: properly shim URL
export class URL {
    constructor(url: string) {
        console.log(url)
    }
}

// TODO: properly shim performance.now()
export const self = {
    performance: {
        now(): number {
            return 0
        },
    },
}

export const window = {
    btoa: (data: string): string => {
        return ""//btoa(data)
    },
    atob: (data: string): string => {
        return ""//atob(data)
    },
}