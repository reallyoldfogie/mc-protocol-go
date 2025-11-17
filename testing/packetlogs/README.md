# Packet Log Test Fixtures

This directory contains curated packet log samples used for testing the round-trip serialization and deserialization of generated packet types.

## Structure

```
packetlogs/
  <version>/
    sample_clientbound.log  # Small sample of clientbound packets for that version
```

Each `.log` file is JSON-lines format, where each line is a `models.PacketLog` entry captured from a real Minecraft connection.

## Running Tests

Tests are located in `testing/packetlog_roundtrip_test.go`:

### Fixture-based test (default, runs in CI)

```bash
go test ./testing -run TestPacketLogs_RoundTrip_Fixtures -v
```

This uses the small curated logs checked into this directory.

### External log test (optional, for local development)

```bash
MC_PACKETLOG_DIR=/path/to/mc-agent/daze/logs \
  go test ./testing -run TestPacketLogs_RoundTrip_FromEnv -v
```

This processes all `.log` files under the specified directory (e.g., from `mc-agent`).

## Known Issues

The fixture test currently identifies real issues in the generated packet code:

### Version 1.21.5

**4 packets fail round-trip in `sample_clientbound.log`:**

1. **ClientboundDeclareRecipes** (id=126): EOF during `Scan` at field[1]
2. **ClientboundDeclareCommands** (id=16): EOF during `Scan` at field[0]
3. **ClientboundRecipeBookAdd** (id=67): EOF during `Scan` at field[0]
4. **ClientboundPosition** (id=65): Data length mismatch after `Marshal` (log=61 bytes, marshal=58 bytes)

**6 packets succeed:**
- ClientboundLogin
- ClientboundDifficulty
- ClientboundAbilities
- ClientboundHeldItemSlot
- ClientboundEntityStatus
- ClientboundRecipeBookSettings

These failures indicate bugs in the generated `ReadFrom`/`Scan` or `WriteTo`/`Marshal` implementations for complex packets. The test framework is working correctly by exposing these issues.

## Maintaining Fixtures

When adding new fixture logs:

1. Capture logs from `mc-agent` with packet logging enabled.
2. Copy a small representative sample (10-50 packets) into `packetlogs/<version>/`.
3. Prefer diverse packet types over many instances of the same packet.
4. Keep files small (< 100KB) to avoid bloating the repo.
5. Update this README if new known issues are discovered.

## Future Work

- Add serverbound fixture logs once mc-agent supports logging outbound packets.
- Add fixtures for login/config/status namespaces (currently only play exists).
- Fix generator issues identified by failing tests, then re-capture clean fixtures.
