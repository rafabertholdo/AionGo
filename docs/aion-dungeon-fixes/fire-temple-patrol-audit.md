# Fire Temple: Silver Blade Rotan patrol geometry audit

The pre-correction route **13**, used by **212844** in world **320100000**, is not
collision-safe in the **1.9.0.1 client**. Restoring patrol scheduling exposed
invalid movement connections that the AI progression regression does not test.

The client level disables rendered terrain. Its collision floors, walls and
rocks are placed CGF meshes; the `land_map.h32` heightmap cannot validate this
instance's movement. This check extracted the client's brush placements and
collision meshes, applied their transforms, and intersected every consecutive
route connection, including the cyclic return, against that geometry.

## Applied correction

Rotan now patrols **back and forth over the same corridor**, following the user's
live-server behavior description. The physical endpoints are the old points
**3** and **12**; his spawn lies between them. The route visits the nearby end,
retraces through the spawn, follows the corridor to the far end, then returns
along those same coordinates. The cyclic XML representation closes at the
spawn on this retraced corridor, never by joining the two physical endpoints.

The correction removes old point **5**, routes around the rock between old
points **3** and **4**, and samples the client collision floor along slopes.
It uses **76** steps spaced approximately **5.2 to 6.3 m** apart. Forward and
return coordinates are symmetric around the endpoints. The corresponding
[correction evidence](fire-temple-patrol-correction.json) records the updated
route hash and validation parameters.

A float32 movement simulation used Rotan's **4.5 m/s** walk speed, the current
mover's **2 m** arrival radius, half-second movement ticks and one-second AI
ticks. Both tick orders visited all steps. Collision rays at heights **0.5,
1.0 and 1.5 m**, on the centre line and **0.5 m** to either side, found **zero**
intersections. Simulated feet differed from the nearby collision floor by
**−0.2518 to +0.2232 m**. These probes are not a full capsule sweep or a
real-client playback verification.

The regression now checks the symmetric return, retained corridor endpoints,
short floor-sampling steps, continued patrol under observation and transition
to combat. Generic chase/return-home movement still lacks a collision engine;
this change corrects Rotan's idle patrol data.

## Confirmed pre-correction findings

| Route connection | Distance | Collision at recorded Z + 1 metre |
| --- | ---: | --- |
| 3 → 4 | 46.610 m | Cave rock, two triangle intersections |
| 4 → 5 | 32.097 m | Invisible `nowalk_obstacle4_08m` wall, two intersections |
| 5 → 6 | 13.793 m | Invisible `nowalk_obstacle4_08m` wall, two intersections |
| 12 → 1 | 91.667 m | Multiple rocks and structural collision meshes, eight intersections |

The other **11 waypoint heights match client collision floors within 0.001 m**.
Point **5**, `(224.38875, 190.98651, 114.34763)`, has no matching floor. Vertical
collision intersections at this XY occur at Z **108.0395**, **109.9478**,
**112.1785**, and **115.6009**; the last is a steep surface. Merely replacing its
Z with the nearest value does not establish a safe passage through the nearby
invisible wall.

Even connections clear at chest height are not proven walkable: low probes
intersect floor geometry along several slopes, and sampled floor heights can
differ from the server's linear Z interpolation by approximately **0.9 m**.
The Go mover interpolates directly toward the next point and currently performs
no client collision or ground-height query. Thus correct endpoints alone do
not establish a safe segment.

The pre-correction probes identified **4 → 6** and return through the previous
corridor points as clear chest-height connections. The correction above adds
floor samples and validates these connections with the mover's arrival radius
and side probes. Retaining the old cycle or lowering point 5 alone would leave
confirmed blocked connections.

## Evidence and reproducibility

The [numeric evidence](fire-temple-patrol-evidence.json) contains the source
archive and route hashes, extractor revision, waypoint floor comparisons,
segment intersections and method limitations. Raw client assets and extracted
meshes stay outside this public repository.

The temporary extractor was based on
[beyond-aion/aion-geobuilder](https://github.com/beyond-aion/aion-geobuilder),
revision `e0ce8375c525c86f264ff93352652bfc4723e682`, with two 1.9 adaptations:
skip the absent housing archive; parse 88-byte brush records (block size 14)
without the later event-type field. Extraction covered all **261** referenced
meshes: **193** contained retained collision geometry, **68** were empty after
collision filtering, and **0** were missing. The transformed scene contains
**107,333** collision triangles.

Each connection was tested in both-sided collision geometry at offsets
**0.1, 0.5, 1.0 and 1.5 m** above its recorded, linearly interpolated Z. Physics,
NPC-walk and physical-see-through collision intentions were included. Floor
probes compare vertical intersections to each waypoint. The audit's slope
filter is not a verified 1.9 walking-slope rule.

This check does not certify body-radius clearance, continuous ground contact,
the client's navigation-file format, or a replacement route. The original audit was read-only. The subsequent correction changes route 13
and its regression test; the client files and running server remain untouched.
