# ClientboundEntityUpdateAttributes' attribute-ID mapper is missing entries, shifting every later attribute's name

## Summary

`EntityUpdateAttributesPropertiesArrayTypeKeyMappings` — the generated
VarInt-to-name mapper for `ClientboundEntityUpdateAttributes`'
`properties[].key` field, in every supported version's
`data/<version>/play/clientbound/packet_mapper.go` — is missing several
attributes that exist in the real game's attribute registry. Because these
IDs are plain sequential registration-order indices (not a stable enum),
every attribute registered **after** the first missing one is off by however
many entries are missing before it, and gets labeled with the **wrong
name**.

This is not a mc-protocol-go generator bug: the generator's own input data
(the cached `protocol.json`, e.g.
`.cache-old/metadata/1.21.11/downloads/protocol.json`) already contains the
same incomplete mapper. The root cause is upstream, in whatever
PrismarineJS/minecraft-data snapshot this protocol.json was sourced from.

**Confirmed live**, not just by source inspection: applying Speed II to a
tracked entity on a real 1.21.11 server produces a
`ClientboundEntityUpdateAttributes` packet whose `Modifiers` array (amount
`0.4`, operation `2` = `ADD_MULTIPLIED_TOTAL` — exactly Speed II's real
modifier, `0.2 * (amplifier+1)`) arrives attached to the key our mapper
labels `"generic.scale"`, not `"generic.movement_speed"`.

## Impact

Every consumer that reads an attribute by name for any attribute registered
at or after `generic.max_health` in the real registry gets the **wrong
attribute's data** for every one of the 11 versions this repo currently
generates data for (1.21.1 through 1.21.11). Concretely, for
`generic.movement_speed` alone: any downstream project reading it (mc-agent
uses it for Speed/Slowness and for resolving ridden-mount movement speed —
horses, camels, striders, etc.) silently reads whichever *other* attribute
happens to have been shifted into that name instead, or gets nothing at all
for attributes shifted past the end of the (too-short) mapper.

Since this is a plain integer-to-string lookup table with no version-range
checks, the mislabeling is silent — no parse error, no panic, just wrong
data (or, in a couple of cases below, the *correct* base value under the
*wrong* modifier, which is even harder to notice than a missing value).

## Root cause: the mapper is missing 4-5 attributes depending on version

Cross-referencing each version's actual attribute registration order —
decompiled directly from
`net/minecraft/entity/attribute/EntityAttributes.java`, not inferred — against
the corresponding generated mapper shows the same shape of gap in every
version: newer attributes (`mining_efficiency`, `movement_efficiency`,
`oxygen_bonus`, `sneaking_speed`, and for 1.21.6+ also `camera_distance` is
present but attributes added *after* it are still missing) are absent from
the mapper entirely, shifting every subsequent index down by the number of
missing entries before it.

### Group A: 1.21.1 – 1.21.4 (identical mapper across all four)

Real registration order (1.21.1's decompiled source uses the *prefixed*
string IDs directly — `"generic.movement_speed"`, etc. — so no prefix
inference is needed for this group):

```
extractedSrc/1.21.1/net/minecraft/entity/attribute/EntityAttributes.java
```

```
 0 generic.armor                        11 generic.flying_speed             22 generic.oxygen_bonus
 1 generic.armor_toughness              12 generic.follow_range             23 generic.safe_fall_distance
 2 generic.attack_damage                13 generic.gravity                  24 generic.scale
 3 generic.attack_knockback             14 generic.jump_strength            25 player.sneaking_speed
 4 generic.attack_speed                 15 generic.knockback_resistance     26 zombie.spawn_reinforcements
 5 player.block_break_speed             16 generic.luck                     27 generic.step_height
 6 player.block_interaction_range       17 generic.max_absorption           28 player.submerged_mining_speed
 7 generic.burning_time                 18 generic.max_health               29 player.sweeping_damage_ratio
 8 generic.explosion_knockback_resist.  19 player.mining_efficiency         30 generic.water_movement_efficiency
 9 player.entity_interaction_range      20 generic.movement_efficiency
10 generic.fall_damage_multiplier       21 generic.movement_speed
```

31 real attributes (0-30). The generated mapper
(`data/1.21.1/play/clientbound/packet_mapper.go`,
`EntityUpdateAttributesPropertiesArrayTypeKeyMappings`, byte-identical in
1.21.2/1.21.3/1.21.4) and its own source `protocol.json` snippet
(`.cache-old/metadata/1.21.1/downloads/protocol.json`) only have **22**
entries (0-21):

```json
{
  "0": "generic.armor", "1": "generic.armor_toughness", "2": "generic.attack_damage",
  "3": "generic.attack_knockback", "4": "generic.attack_speed", "5": "player.block_break_speed",
  "6": "player.block_interaction_range", "7": "player.entity_interaction_range",
  "8": "generic.fall_damage_multiplier", "9": "generic.flying_speed", "10": "generic.follow_range",
  "11": "generic.gravity", "12": "generic.jump_strength", "13": "generic.knockback_resistance",
  "14": "generic.luck", "15": "generic.max_absorption", "16": "generic.max_health",
  "17": "generic.movement_speed", "18": "generic.safe_fall_distance", "19": "generic.scale",
  "20": "zombie.spawn_reinforcements", "21": "generic.step_height"
}
```

Missing entirely: `generic.burning_time`,
`generic.explosion_knockback_resistance` (both dropped right after index 6,
a 2-entry gap), then `player.mining_efficiency`,
`generic.movement_efficiency`, `generic.oxygen_bonus`,
`player.sneaking_speed`, `player.submerged_mining_speed`,
`player.sweeping_damage_ratio`, `generic.water_movement_efficiency` (7 more,
after `max_health`) — 9 missing total, which is why the mapper tops out at
21 instead of 30.

Real wire ID 21 (`generic.movement_speed`) is read through this mapper as
key 21 → **`"generic.step_height"`**. Real wire IDs 22-30 (`oxygen_bonus`
through `water_movement_efficiency`) fall outside the mapper's range
entirely and fail as unknown keys.

### Group B: 1.21.5 (its own, differently-sized mapper)

Real order (32 attributes, 0-31 — same as Group A plus `generic.tempt_range`
inserted before `water_movement_efficiency`; confirmed via
`extractedSrc/1.21.5/.../EntityAttributes.java`, which by this version has
already dropped the string-ID prefixes, so prefixes below are inferred from
Group A's confirmed categorization, not re-derived from 1.21.5's own source
text):

```
 0 generic.armor  … (same as Group A through index 18: generic.max_health)
19 player.mining_efficiency     24 generic.scale                29 player.sweeping_damage_ratio
20 generic.movement_efficiency  25 player.sneaking_speed        30 generic.tempt_range
21 generic.movement_speed       26 zombie.spawn_reinforcements  31 generic.water_movement_efficiency
22 generic.oxygen_bonus         27 generic.step_height
23 generic.safe_fall_distance   28 player.submerged_mining_speed
```

Generated mapper / `protocol.json` (`.cache-old/metadata/1.21.5/downloads/protocol.json`) has 27 entries (0-26):

```json
{
  "0": "generic.armor", "1": "generic.armor_toughness", "2": "generic.attack_damage",
  "3": "generic.attack_knockback", "4": "generic.attack_speed", "5": "player.block_break_speed",
  "6": "player.block_interaction_range", "7": "camera_distance", "8": "explosion_knockback_resistance",
  "9": "player.entity_interaction_range", "10": "generic.fall_damage_multiplier", "11": "generic.flying_speed",
  "12": "generic.follow_range", "13": "generic.gravity", "14": "generic.jump_strength",
  "15": "generic.knockback_resistance", "16": "generic.luck", "17": "generic.max_absorption",
  "18": "generic.max_health", "19": "generic.movement_speed", "20": "generic.safe_fall_distance",
  "21": "generic.scale", "22": "zombie.spawn_reinforcements", "23": "generic.step_height",
  "24": "submerged_mining_speed", "25": "sweeping_damage_ratio", "26": "tempt_range"
}
```

Note this version's mapper also has `"camera_distance"` at index 7 —
**an attribute that doesn't exist yet in 1.21.5's real registry at all**
(confirmed: it's absent from 1.21.5's decompiled `EntityAttributes.java`
entirely; it's real starting 1.21.6). That entry appears to have been pulled
from a different version's schema by mistake, independent of the
missing-attributes gap described above, and further confirms this mapper
wasn't generated from 1.21.5's own real registry order.

Missing: `generic.burning_time` (replaced by the bogus `camera_distance` at
the same slot), then the same `mining_efficiency` /
`movement_efficiency` / `oxygen_bonus` / `sneaking_speed` /
`submerged_mining_speed` / `sweeping_damage_ratio` /
`water_movement_efficiency` gap pattern as Group A (7 entries) — 8 missing
total (32 real − 27 mapped = 5 net, since one bogus substitution masks part
of the count; treat the two issues — the missing entries and the bogus
`camera_distance` substitution — as separate defects).

Real wire ID 21 (`generic.movement_speed`) is read through this mapper as
key 21 → **`"generic.scale"`**.

### Group C: 1.21.6 – 1.21.11, and 26.1 (identical mapper across all seven)

Real order (35 attributes, 0-34 — Group B's 32 plus `generic.camera_distance`
now legitimately real, inserted after `burning_time`, plus
`waypoint_transmit_range`/`waypoint_receive_range` appended at the end;
confirmed via `extractedSrc/1.21.11/.../EntityAttributes.java`, prefixes for
the two waypoint attributes not independently confirmable the same way since
no version in this range still uses prefixed string IDs — flagged below):

```
 0 generic.armor  … (same as Group A/B through index 19: generic.max_health)
20 player.mining_efficiency     25 generic.scale                30 player.submerged_mining_speed
21 generic.movement_efficiency  26 player.sneaking_speed        31 player.sweeping_damage_ratio
22 generic.movement_speed       27 zombie.spawn_reinforcements  32 generic.tempt_range
23 generic.oxygen_bonus         28 generic.step_height          33 generic.water_movement_efficiency
24 generic.safe_fall_distance   29 (see 30, shifted)            34 ?.waypoint_transmit_range
                                                                 35 ?.waypoint_receive_range
```

(`camera_distance` sits at real index 8, between `burning_time` at 7 and
`explosion_knockback_resistance` at 9 — included correctly in this group's
mapper, unlike Group B's bogus placement.)

Generated mapper / `protocol.json`
(`.cache-old/metadata/1.21.11/downloads/protocol.json`, byte-identical
through at least 1.21.6-1.21.11) has 31 entries (0-30):

```json
{
  "0": "generic.armor", "1": "generic.armor_toughness", "2": "generic.attack_damage",
  "3": "generic.attack_knockback", "4": "generic.attack_speed", "5": "player.block_break_speed",
  "6": "player.block_interaction_range", "7": "burning_time", "8": "camera_distance",
  "9": "explosion_knockback_resistance", "10": "player.entity_interaction_range",
  "11": "generic.fall_damage_multiplier", "12": "generic.flying_speed", "13": "generic.follow_range",
  "14": "generic.gravity", "15": "generic.jump_strength", "16": "generic.knockback_resistance",
  "17": "generic.luck", "18": "generic.max_absorption", "19": "generic.max_health",
  "20": "generic.movement_speed", "21": "generic.safe_fall_distance", "22": "generic.scale",
  "23": "zombie.spawn_reinforcements", "24": "generic.step_height", "25": "submerged_mining_speed",
  "26": "sweeping_damage_ratio", "27": "tempt_range", "28": "water_movement_efficiency",
  "29": "waypoint_transmit_range", "30": "waypoint_receive_range"
}
```

Missing: the same 4 (`mining_efficiency`, `movement_efficiency`,
`oxygen_bonus`, `sneaking_speed`) after `max_health` — a clean, single,
uniform 4-entry gap for this group (unlike Groups A/B's multiple gaps),
which is exactly why every real index from `max_health` (19) onward reads
4 slots low in the current mapper.

Real wire ID 22 (`generic.movement_speed`) is read through this mapper as
key 22 → **`"generic.scale"`**.

**26.1 confirmed part of this group** (2026-08-24, source inspection only —
no live server available for a brand-new version): 26.1's generated mapper,
`data/26.1/play/clientbound/packet_mapper.go`'s
`EntityUpdateAttributesPropertiesArrayTypeKeyMappings`, is the same 31-entry
table (0-30) as 1.21.6-1.21.11's, byte-identical apart from Go map literal
key ordering (which is unordered/non-deterministic per `go fmt` and carries
no meaning). Its source `protocol.json`
(`.cache/metadata/26.1/downloads/protocol.json`, packet
`entity_update_attributes`, the `key` field's `mappings`) is likewise
byte-identical to Group C's table.

26.1's real attribute registration order was cross-referenced against
`mc-data-gen/extractedSrc/26.1/net/minecraft/world/entity/ai/attributes/Attributes.java`
(note: the attribute registry class moved packages/renamed between 1.21.11
and 26.1 - from `net.minecraft.entity.attribute.EntityAttributes` to
`net.minecraft.world.entity.ai.attributes.Attributes` - but the
`register(name, attribute)` call order, and thus the wire index assignment,
is identical to Group C's documented real order): same 35 attributes (0-34),
same single 4-entry gap right after `max_health` (19) for
`mining_efficiency`, `movement_efficiency`, `oxygen_bonus`, `sneaking_speed`
(indices 20-23 real, all four absent from the generated mapper), same
`generic.movement_speed` at real index 22 landing on the mapper's
`"generic.scale"` label. No new attributes were added and none were
reordered relative to 1.21.6-1.21.11's real order.

Conclusion: 26.1 inherits Group C's bug unchanged. mc-agent's
`handler_versions/v26_1/entities.go` carries the same
`correctAttributeKey("generic.scale" -> "generic.movement_speed")`
workaround as `handler_versions/v1_21_11/entities.go`, copied verbatim
(only package/import names adapted).

**Live confirmation** (2026-08-22, real 1.21.11 Docker server, via
mc-agent's `testing/attribute_modifier_test.go`): applying
`effect give <player> minecraft:speed 30 1` produced

```
[onEntityUpdateAttributes] Updated attributes for entity 2:
  map[generic.scale:{0.10000000149011612 [{0.4000000059604645 2}]}]
```

Base value `0.1` is vanilla's real player `generic.movement_speed` default;
modifier `{0.4, operation 2}` is exactly Speed II's real
`ADD_MULTIPLIED_TOTAL` modifier (`0.2 * (amplifier+1)`, amplifier=1,
`StatusEffects.java`). Both are correct — only the *key* is wrong.

## Recommended fix

Add the missing attributes back into each affected version's mapper data in
their correct real registration-order position (shifting every subsequent
index up to make room), sourced from whatever protocol.json / minecraft-data
snapshot mc-protocol-go's generator consumes for each version. The
per-group real orderings above give the exact target sequence. Two items to
resolve before submitting a patch:

1. **1.21.5's bogus `camera_distance` entry** (index 7) needs removing
   independently of the missing-attributes fix — it's not a real attribute
   for that version at all, not just misplaced.
2. **The naming convention for `waypoint_transmit_range` /
   `waypoint_receive_range`** (Group C, indices 33-34) isn't independently
   confirmed the way the others are, since no version in this codebase's
   supported range still uses prefixed attribute string IDs the way 1.21.1
   does. Worth checking Mojang's own current source or another reference
   before asserting `generic.` for these two.

## Downstream workaround (temporary)

mc-agent has a **temporary, local** correction layer scoped to just
`generic.movement_speed` (the only attribute it currently reads that's
affected) in `handler_versions/<version>/entities.go`'s
`correctAttributeKey` function — remap the wrong label the *current* mapper
produces (`generic.step_height` for Group A, `generic.scale` for Groups
B/C) back to `generic.movement_speed`. This does **not** fix the other
attributes this same gap affects (`oxygen_bonus`, `scale`, `step_height`,
`submerged_mining_speed`, `sweeping_damage_ratio`, etc.) — those remain
silently wrong in mc-agent until this is fixed upstream and the workaround
is deleted.

## Verification once fixed

- Re-run mc-agent's `testing/attribute_modifier_test.go`
  (`TestSpeedEffectModifiesTrackedAttribute`) across all 11 versions with
  `correctAttributeKey` removed — it should still pass, reading
  `generic.movement_speed` directly with no correction needed.
- Spot-check a few of the other affected attributes
  (`generic.scale`, `generic.step_height`, `generic.oxygen_bonus`) against a
  real server the same way, to confirm the fix isn't movement_speed-specific.
