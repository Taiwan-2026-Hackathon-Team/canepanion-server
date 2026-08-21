import assert from 'node:assert/strict'
import { test } from 'node:test'
import { manhattan, pointAtLength, polylineLengths, routeToPolyline } from './iso'

test('pointAtLength on a one-point polyline stays on that point', () => {
  const pts = routeToPolyline(manhattan({ gx: 4, gy: 7 }, { gx: 4, gy: 7 }))
  const { cum } = polylineLengths(pts)
  const at = pointAtLength(pts, cum, 0)
  assert.equal(pts.length, 1)
  assert.deepEqual(at, pts[0])
  assert.ok(Number.isFinite(at.x) && Number.isFinite(at.y))
})
