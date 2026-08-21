import * as esbuild from 'esbuild'
import { createRequire } from 'node:module'
import path from 'node:path'
import { repoRoot } from './repo-root.mjs'

const require = createRequire(import.meta.url)
const reactRoot = path.dirname(require.resolve('react/package.json'))
const ROOT = repoRoot()

const alias = {
  react: reactRoot,
  'react/jsx-runtime': require.resolve('react/jsx-runtime'),
  'react-dom/client': require.resolve('react-dom/client'),
  'react-dom': path.dirname(require.resolve('react-dom/package.json')),
  scheduler: require.resolve('scheduler'),
}

await esbuild.build({
  entryPoints: [path.join(ROOT, 'docs/maps/cane-camera/entry.tsx')],
  bundle: true,
  minify: true,
  format: 'iife',
  outfile: path.join(ROOT, 'docs/cane-camera.bundle.js'),
  loader: { '.css': 'text' },
  jsx: 'automatic',
  alias,
  logLevel: 'info',
})
