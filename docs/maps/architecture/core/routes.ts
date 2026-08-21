import { portAnchor } from './archetypes'
import {
  manhattan,
  polylineLengths,
  routeToPolyline,
  type GridPt,
  type ScreenPt,
} from './iso'
import type { ArchEdge, ArchNode } from './types'

/**
 * Every edge, as a drawn line — computed once at module load.
 *
 * One table rather than a computation per layer, because the edge layer, the
 * payload dots and the step badges all have to agree on where a line *is*, and
 * three call sites deriving it separately is three chances to disagree by a
 * pixel. Arc lengths are precomputed too, so placing a dot along a route is
 * arithmetic rather than a `getPointAtLength` call against the DOM.
 */

export type EdgeGeometry = { pts: ScreenPt[]; cum: number[]; total: number }

/** A rectangle in front of the port so a self-call has a path the payload can travel. */
function selfLoop(anchor: GridPt): GridPt[] {
  const dx = 0.7
  const dy = 1.2
  return [
    anchor,
    { gx: anchor.gx + dx, gy: anchor.gy },
    { gx: anchor.gx + dx, gy: anchor.gy + dy },
    { gx: anchor.gx - dx, gy: anchor.gy + dy },
    { gx: anchor.gx - dx, gy: anchor.gy },
    anchor,
  ]
}

function routePoints(from: ArchNode, to: ArchNode, via: GridPt[]): ScreenPt[] {
  const start = portAnchor(from.footprint)
  const end = portAnchor(to.footprint)
  if (from.id === to.id && via.length === 0) return routeToPolyline(selfLoop(start))
  return routeToPolyline(manhattan(start, end, via))
}

export function buildEdgeGeometry(
  nodes: readonly ArchNode[],
  edges: readonly ArchEdge[],
): ReadonlyMap<string, EdgeGeometry> {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const out = new Map<string, EdgeGeometry>()

  for (const edge of edges) {
    const from = byId.get(edge.from)
    const to = byId.get(edge.to)
    // An edge naming a module that does not exist is authored drift, not a
    // crash: skip it and let the graph test report it by name.
    if (!from || !to) continue

    const pts = routePoints(from, to, edge.via ?? [])
    const { cum, total } = polylineLengths(pts)
    out.set(edge.id, { pts, cum, total })
  }
  return out
}

/** Where the arrowhead sits, and which way it points. */
export function arrowhead(geom: EdgeGeometry): { at: ScreenPt; angle: number } | null {
  const pts = geom.pts
  if (pts.length < 2) return null
  const end = pts[pts.length - 1]
  const before = pts[pts.length - 2]
  const angle = (Math.atan2(end.y - before.y, end.x - before.x) * 180) / Math.PI
  return { at: end, angle }
}
