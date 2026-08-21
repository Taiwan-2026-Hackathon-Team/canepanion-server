import { existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

/** Go repo root (directory that contains go.mod), with a trailing slash. */
export function repoRoot() {
  if (process.env.ARCH_ROOT) {
    const root = process.env.ARCH_ROOT
    return root.endsWith('/') ? root : `${root}/`
  }
  let dir = dirname(fileURLToPath(import.meta.url))
  while (true) {
    if (existsSync(join(dir, 'go.mod'))) return `${dir}/`
    const parent = dirname(dir)
    if (parent === dir) throw new Error('architecture scripts: go.mod not found')
    dir = parent
  }
}
