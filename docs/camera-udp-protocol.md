# Cane camera: UDP wire protocol and server ingest

How camera frames get from the cane to the WebRTC relay, and why the design is
shaped the way it is.

The firmware does not speak WebRTC. It sends **raw RGB565 frames over plain
UDP** in a custom slice format. `cmd/caneingest` terminates that protocol and
republishes the video as H.264 over WHIP, so the relay and the guardian app
never see any of it.

```
cane ──UDP slices (RGB565)──► caneingest ──raw frames──► ffmpeg ──H.264──► WHIP ──► relay ──► app
     ◄──── CAMR reports, 1 Hz ────
```

Firmware source of truth: `app/src/main.c` in `cane-panion-firmware`.
Server consumer: `internal/caneingest/`, driven by `cmd/caneingest`.

---

## 1. Session establishment

There is no handshake, no registration, and no authentication on the UDP link.

- The cane binds **UDP port 5000** and captures continuously from boot,
  discarding frames while nobody is watching.
- **Any datagram** sent to that port marks the sender as the viewer. The cane
  then streams to that source address.
- The most recent sender wins. Only **one viewer at a time** — running the
  bench viewer (`tools/cam_view.py`) and `caneingest` together makes them fight
  over the stream.
- If nothing arrives from the viewer for **5 seconds**, the cane declares it
  gone and stops sending.

Because the cane replies to whoever spoke, the ingest needs no firmware change:
it simply plays the viewer role.

> **This link is unauthenticated and unencrypted.** Anyone who can reach port
> 5000 can take over the stream. It is a LAN-only protocol; see §8.

## 2. Slice header

A 160x120 RGB565 frame is 38,400 bytes — far larger than one datagram — so each
frame is cut into slices of at most **1400 payload bytes**, giving 28 slices per
frame. Every slice carries the **full descriptor**, so slices can be processed
in any order and a lost one costs pixels rather than synchronisation.

The header is 26 bytes, little-endian, packed with no padding
(`struct slice_header ... __packed`):

| Offset | Size | Field | Notes |
| --- | --- | --- | --- |
| 0 | 4 | `magic` | ASCII `CAMU`. Anything else is not a camera slice. |
| 4 | 4 | `frame` | u32 frame counter, starts at 1, increments per frame sent. |
| 8 | 2 | `width` | u16 pixels, e.g. 160. |
| 10 | 2 | `height` | u16 pixels, e.g. 120. |
| 12 | 4 | `fourcc` | `0x50424752` = `'RGBP'` = `VIDEO_PIX_FMT_RGB565`. |
| 16 | 4 | `offset` | Byte offset of this slice within the frame. |
| 20 | 4 | `total` | Total bytes in the whole frame (38400 at 160x120). |
| 24 | 2 | `len` | Payload bytes in this slice, ≤ 1400. |
| 26 | `len` | payload | Raw pixel bytes. |

Equivalent Python format string: `struct.Struct("<4sIHHIIIH")`.

A golden slice — frame 7, 160x120, offset 16, 2 payload bytes:

```
43 41 4d 55 07 00 00 00 a0 00 78 00 52 47 42 50
10 00 00 00 00 96 00 00 02 00 ab cd
```

A receiver must reject a datagram when it is shorter than 26 bytes, the magic
is wrong, `total` is 0, `offset + len > total` (compute in 64-bit so a corrupt
offset cannot wrap), or the datagram is shorter than `26 + len`.

## 3. Slices are sent interleaved, not in address order

Within a frame the cane emits **every 4th slice per pass**, over 4 passes —
0, 4, 8, … then 1, 5, 9, … and so on.

This matters on a lossy link. In address order the lost slices land as one
contiguous stale band and the picture reads as a slow wipe down the screen.
Interleaved, the same number of survivors spreads evenly and the whole image
refreshes at once, degrading uniformly.

Slices still go out **back to back** within a pass. Do not add spacing to be
"gentle" on the radio: 802.11 aggregates consecutive packets into one AMPDU,
and inserting gaps was measured to drop delivery from 5.1 to 1.4 fps.

If the WiFi TX pool is exhausted, `sendto` returns `ENOMEM`/`EAGAIN`; the cane
backs off briefly and then **abandons the rest of that frame** rather than let
one congested moment stall the stream.

## 4. Reassembly: paint a canvas, don't wait for whole frames

The receiver keeps two things, for two different purposes.

**A persistent canvas** — a single frame-sized buffer, painted slice by slice
and never cleared. This is what gets encoded and shown. On a bad link *no*
frame may ever arrive complete (at 55% slice loss the odds of all 28 slices
landing are about 1 in 10⁹), so insisting on whole frames would display almost
nothing. Painting into a canvas turns a lost slice into a **stale band**, not a
missing frame.

**Per-frame buffers** — used only to count frames that arrived genuinely whole.
That count is fed back to the cane and must not be flattered, because its rate
controller hill-climbs on it (§5). Frames older than 4 behind the newest are
evicted so a slice that never arrives cannot leak a buffer forever.

The canvas is handed to the encoder on a **fixed tick**, independent of what
has arrived. The encoder needs a steady cadence, and unchanged regions cost
almost nothing to encode.

## 5. Feedback: the `CAMR` report

Once per second the viewer sends a 12-byte report back to the cane. It doubles
as the keepalive that stops the 5-second viewer timeout.

| Offset | Size | Field |
| --- | --- | --- |
| 0 | 4 | ASCII `CAMR` |
| 4 | 4 | u32 slices received **since the last report** |
| 8 | 4 | u32 whole frames completed **since the last report** |

Both counters are **deltas**, not totals.

The cane uses `frames` to hill-climb its send rate between **2 and 30 fps** in
steps of 2, starting from `CONFIG_CANE_CAM_TARGET_FPS` (default 15).

It optimises **whole frames delivered per second, deliberately not delivery
ratio.** Those disagree: sending 14 fps got 4.5–7.9 whole frames through at a
~30% success rate, while backing off to 3 fps raised success to 81% and
delivered only 2.4. Losses behave like a roughly fixed per-packet probability
rather than a cliff, so pushing harder wins until it stops winning.

A viewer that sends bare keepalives instead of real reports leaves the cane
pinned at its compiled-in default rate.

## 6. What the server does with it

`cmd/caneingest` runs as its own process, so frames never pass through the Gin
handlers — consistent with the principle in `api.md` that large media should
not be proxied through the application server.

1. Exchange the device credential for a token via `POST /api/v1/firmware/session`.
2. Announce to the cane, then absorb slices and send `CAMR` reports at 1 Hz.
3. Reassemble into the persistent canvas. **Frame size is read off the wire**,
   not configured.
4. Feed the canvas to `ffmpeg` at a fixed rate: `rawvideo`/`rgb565le` in,
   H.264 constrained baseline out, one slice per frame, SPS/PPS repeated before
   every keyframe.
5. Publish to the relay over WHIP using **pion**, one sample per access unit.

**It deliberately does not use FFmpeg's WHIP muxer**, which cannot satisfy this
relay on three counts: it offers `a=setup:passive` where `actpass` is required,
inlines no ICE candidates where at least one is required, and zeroes the
`profile_iop` byte so it advertises `profile-level-id=4200xx` regardless of what
the encoder actually produced. FFmpeg is therefore used purely as a codec.

Configuration is documented in `api.md` under *Ingesting the cane's UDP camera
stream*.

## 7. Measured behaviour

Sensor capture is **42.8 fps** at 160x120 and has never been the bottleneck.
The link is.

| Link | Sent | Delivered | Slice loss |
| --- | --- | --- | --- |
| Home AP (earlier) | 17–21 fps | 5.8–10.7 fps | ~60% |
| iPhone hotspot, 2.4 GHz ch.6 | 21.5 fps | 21.4 fps | ~0% |

On the hotspot the cane reported `3024 slices ok / 0 refused` while the ingest
counted 3052 slices in the adjacent window, and the rate controller climbed to
its 30 fps ceiling. At ~21 fps the raw stream is roughly **820 kB/s (6.6 Mbps)**.

Measurements minutes apart are not comparable — the same firmware has been seen
at 308 kB/s and 130 kB/s in one session. Re-measure the baseline in the same
sitting before believing any config change.

## 8. Known limits

- **No authentication, no encryption, single viewer.** LAN only.
- **Uncompressed on the wire.** 160x120 RGB565 at 10 fps is 3.07 Mbps and
  691 MB/hour — far past what the final board's SIM7670G (LTE Cat-1bis) can
  carry, and unaffordable as cellular data. Cellular needs on-device JPEG plus
  device-initiated registration to get through carrier NAT.
- **Counters cannot see a corrupted picture.** Frames captured, slices sent and
  delivered can all read perfectly healthy while the image is rainbow noise.
  Set `CANE_DEBUG_PNG` and look at a frame. The cheap automated proxy is
  *roughness* — mean absolute luma difference between horizontally adjacent
  pixels — which corruption drives up by about an order of magnitude. A very
  **low** roughness with few distinct pixel values means the opposite: a valid
  but near-black image, i.e. a covered lens or an unlit room.
- **A stalled canvas is not a live camera.** The ingest tears the publication
  down after 10 seconds without new slices rather than let the relay report
  `LIVE` over a frozen picture.

## 9. Related

- `api.md` — WHIP/WHEP relay endpoints and ingest configuration
- `internal/caneingest/reassembler.go` — the header parser and canvas, with tests
- `cmd/whepprobe` — subscribes as a real viewer and counts RTP, which is how to
  tell a flowing stream from a publication that merely exists
- `tools/whep-viewer.html` — browser viewer, with decoder stats and a
  brightness boost for dark scenes
