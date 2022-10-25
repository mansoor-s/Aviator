![Aviator Logo  ](/static_assets/logo1.png "Aviator logo")


Aviator lets your application serve server-side rendered (SSR) Svelte in Go. Think [SvelteKit](https://kit.svelte.dev/) but in Go. Aviator is only concerned with rendering Svelte pages and doesn't know anything about serving HTTP requests so users are free to use their favorite HTTP libraries. It also doesn't force any Go project structure on the user.


One of the main goals of the project is to be able to use any npm package. Most packages work, but some packages that use some ES6 features might not work (see below for list of known issues).


Aviator is **pre-beta quality** and not suitable for production use yet.


### Known Issues And Roadmap ###
* SSR errors could be much better. They should include stack traces
* Only a dev build of JS and CSS assets are currently created which are unoptimized, not minified, and contain sourcemaps
* Svelte compiler is currently bundled with Aviator, that means the library user must use the same version in it's package.json. In the future, the svelte compiler will be built on application startup and cached.
* Support for CSS pre-processors (tailwindcss, scss, postcss)
* Typescript is support but not inside of .svelte files due to lack of TS pre-processor for svelte
* Support for some ES6 features (modules, generators, async/await) - These will be added as soon as they're added into [Goja](https://github.com/dop251/goja) 