import * as esbuild from 'esbuild'
import { createRequire } from 'node:module'
import path from 'node:path'

const require = createRequire(import.meta.url)
const reactRoot = path.dirname(require.resolve('react/package.json'))

const alias = {
  react: reactRoot,
  'react/jsx-runtime': require.resolve('react/jsx-runtime'),
  'react-dom/client': require.resolve('react-dom/client'),
  'react-dom': path.dirname(require.resolve('react-dom/package.json')),
  scheduler: require.resolve('scheduler'),
}

await esbuild.build({
  entryPoints: ['src/architecture-webrtc/entry.tsx'],
  bundle: true,
  minify: true,
  format: 'iife',
  outfile: 'docs/cane-camera.bundle.js',
  loader: { '.css': 'text' },
  jsx: 'automatic',
  alias,
  logLevel: 'info',
})
