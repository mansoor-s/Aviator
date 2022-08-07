#!/usr/bin/env bash

go run github.com/evanw/esbuild/cmd/esbuild compiler.ts --format=iife \
--global-name=__svelte__ --bundle --platform=node --inject:shimssr.ts \
--external:url --outfile=svelte_compiler.js --log-level=warning