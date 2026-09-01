# EnchantItem.Enchantment (button ID) has the wrong wire type for 1.21.11 only

## Summary

`data/1.21.11/play/serverbound/packet_enchantitem.go`'s `Enchantment` field
(the button ID for `ServerboundContainerButtonClickPacket`/`EnchantItem` —
used for enchanting table option selection, stonecutter recipe selection,
and any other button-based container UI) is generated as `pk.Byte` (`i8`).
Every other version this repo generates data for — 1.21.1 through 1.21.10,
and the newly-added 26.1 — generates it as `pk.VarInt` (`varint`), and the
real game has always used varint for this field, in every version checked
including 1.21.11 itself.

This is not a mc-protocol-go generator bug: the generator's own input data
(`.cache/metadata/1.21.11/downloads/protocol.json`) already declares this
field `"i8"` for 1.21.11 only. The root cause is upstream, in whatever
PrismarineJS/minecraft-data snapshot 1.21.11's protocol.json was sourced
from — an isolated bad entry for this one version, not a real wire-format
change or transition.

## Confirmed against decompiled source, both endpoints and 26.1

`ButtonClickC2SPacket.buttonId`'s codec is identical across every version
checked:

```
extractedSrc/1.21.1/net/minecraft/network/packet/c2s/play/ButtonClickC2SPacket.java:
    PacketCodecs.SYNC_ID, ButtonClickC2SPacket::syncId,
    PacketCodecs.VAR_INT, ButtonClickC2SPacket::buttonId, ButtonClickC2SPacket::new

extractedSrc/1.21.5/net/minecraft/network/packet/c2s/play/ButtonClickC2SPacket.java:
    PacketCodecs.SYNC_ID, ButtonClickC2SPacket::syncId,
    PacketCodecs.VAR_INT, ButtonClickC2SPacket::buttonId, ButtonClickC2SPacket::new

extractedSrc/1.21.11/net/minecraft/network/packet/c2s/play/ButtonClickC2SPacket.java:
    PacketCodecs.SYNC_ID, ButtonClickC2SPacket::syncId,
    PacketCodecs.VAR_INT, ButtonClickC2SPacket::buttonId, ButtonClickC2SPacket::new
```

`buttonId` is `PacketCodecs.VAR_INT` in 1.21.11's own decompiled source —
the same version whose `protocol.json` claims `i8`. The newest version
added this session (26.1, Mojang mappings rather than Yarn, hence the
different class/package name) agrees:

```
mc-protocol-go/.cache/metadata/26.1/extracted_client/.../ServerboundContainerButtonClickPacket.class
  → decompiled: extractedSrc/26.1/net/minecraft/network/protocol/game/ServerboundContainerButtonClickPacket.java
    ByteBufCodecs.CONTAINER_ID, ServerboundContainerButtonClickPacket::containerId,
    ByteBufCodecs.VAR_INT,      ServerboundContainerButtonClickPacket::buttonId,
    ServerboundContainerButtonClickPacket::new
```

So the field has been `varint` in every version before 1.21.11, in
1.21.11 itself (per its own decompiled source), and in 26.1 after it —
1.21.11's `protocol.json` is the single outlier.

## protocol.json comparison across all 12 generated versions

```
1.21.1  – 1.21.10: {"name": "enchantment", "type": "varint"}   (10/10 identical)
1.21.11:            {"name": "enchantment", "type": "i8"}       (outlier)
26.1:               {"name": "enchantment", "type": "varint"}
```

Source files: `.cache/metadata/<version>/downloads/protocol.json`,
`play.toServer.types.packet_enchant_item`.

## Generated Go code

```
data/1.21.1/play/serverbound/packet_enchantitem.go:  Enchantment pk.VarInt
data/1.21.10/play/serverbound/packet_enchantitem.go: Enchantment pk.VarInt
data/1.21.11/play/serverbound/packet_enchantitem.go: Enchantment pk.Byte   // wrong
data/26.1/play/serverbound/packet_enchantitem.go:    Enchantment pk.VarInt
```

## Impact

For 1.21.11 only, `EnchantItem.WriteTo`/`Marshal` serializes the button ID
as a single raw byte instead of a varint. A varint's continuation bit is
clear for values 0-127, making that encoding byte-identical to a plain
byte on the wire, so a value in that range round-trips correctly either
way. It would corrupt the packet stream for a button ID ≥128 (the real
`ButtonClickC2SPacket.buttonId` is a full `varint`, i.e. effectively
`int32`-ranged, matching e.g. a stonecutter menu with more than 128
matching recipes).

mc-agent's own `models.ContainerHandler.SendContainerButtonClick` interface
(`models/version_handler.go`) currently types `buttonID` as `int8`, which
caps every caller in this codebase at 127 — so in mc-agent's *current*
architecture this can't yet manifest as real wire corruption. It's still a
genuine defect in the generated 1.21.11 code (inconsistent with every
other version and the real protocol), and would become a live bug the
moment that interface is ever widened to match the protocol's actual
range, or for any other consumer of mc-protocol-go's 1.21.11 data that
isn't similarly constrained.

## Recommended fix

Correct 1.21.11's `protocol.json` entry for `packet_enchant_item`'s
`enchantment` field from `"i8"` to `"varint"`, matching every other
version and the decompiled source, then regenerate 1.21.11's data.

## Downstream workaround (temporary)

mc-agent's `handler_versions/v1_21_11/containers.go` `SendContainerButtonClick`
no longer uses the generated `EnchantItem` struct's `Enchantment` field
(and thus the generated `Marshal()`/`WriteTo()` byte-truncating encoder) at
all. It hand-assembles the packet with `pk.Marshal(pkt.PacketID(),
&pkt.WindowId, pk.VarInt(buttonID))`, writing a real varint in the
field's place regardless of the generated struct's (wrong) type. Revert to
calling `pkt.Marshal()` directly once this is fixed upstream and 1.21.11's
data is regenerated.

## Verification once fixed

- Regenerate 1.21.11's data and confirm
  `data/1.21.11/play/serverbound/packet_enchantitem.go` declares
  `Enchantment pk.VarInt`.
- Revert `handler_versions/v1_21_11/containers.go`'s
  `SendContainerButtonClick` to `pkt.Enchantment = pk.VarInt(buttonID)` +
  `pkt.Marshal()`, matching the other 10 version handlers.
- Exercise a button-based container UI (enchanting table, stonecutter) on
  a live 1.21.11 server and confirm the server accepts the button click as
  before.
