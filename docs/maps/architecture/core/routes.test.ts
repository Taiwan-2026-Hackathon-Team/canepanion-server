import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { ArchNode } from './types'
import { buildEdgeGeometry } from './routes'

function stub(id: string): ArchNode {
  return {
    id,
    code: 'X',
    name: id,
    role: id,
    group: 'g',
    archetype: 'cube',
    footprint: { gx: 0, gy: 0, w: 2, d: 2 },
    height: 2,
    whatItDoes: '',
    howItsBuilt: '',
    files: [],
  }
}

test('a self-edge is a loop with length, not a single point', () => {
  const node = stub('camera')
  const geometry = buildEdgeGeometry(
    [node],
    [{ id: 'loop', from: 'camera', to: 'camera', kind: 'call', label: 'forward RTP', flowIds: [] }],
  )
  const geom = geometry.get('loop')
  assert.ok(geom)
  assert.ok(geom.pts.length > 2)
  assert.ok(geom.total > 0)
})
