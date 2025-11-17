# Plan: Packet Log–Driven Round-Trip Testing for Generated Packets

## Status

- **Status:** Phase 1 and Phase 2 complete (refactored to CLI tool); Phase 3-5 pending
- **Last Updated:** 2025-11-14
- **Test Results:** Core library functional with 6 unit tests passing; standalone CLI tool validates real packet logs and identifies multiple generator bugs

## Overview

`mc-agent` now writes JSON-line logs of raw packets that conform to `models.PacketLog` in `mc-protocol-go`:

```json
{
  "id": 43,
  "name": "ClientboundLogin",
  "timestamp": "2025-11-14T10:58:49.151950825-07:00",
  "version": "1.21.5",
  "protocol_version": 770,
  "data": "AADthAADE21pbmVjcmFmdDpvdmVyd29ybGQRbWluZWNyYWZ0OnRoZV9lbmQUbWluZWNyYWZ0OnRoZV9uZXRoZXIIDAwAAQAAE21pbmVjcmFmdDpvdmVyd29ybGT8VIGXmuuVVgD/AAAAAD8A"
}
```

This structure corresponds to:

```go
type PacketLog struct {
    ID              int32     `json:"id"`
    Name            string    `json:"name"`
    Timestamp       time.Time `json:"timestamp"`
    Data            []byte    `json:"data"`
    Version         string    `json:"version"`
    ProtocolVersion uint      `json:"protocol_version"`
}
```

Each `data` field is the raw packet bytes (base64-encoded in JSON) as seen by `mc-agent`.

**Goal:** Build a reusable framework in `mc-protocol-go` that replays these logs using the generated packet types, to validate:

- `Scan` / `Marshal` (via `models.PacketMarshaller`)
- `ReadFrom` / `WriteTo` for the same structs

and eventually cover **both clientbound and serverbound** packets across all namespaces (handshaking, status, login, configuration, play).

---

## Current State

### Packet logging (mc-agent)

- Logs live under `mc-agent/daze/logs/`, e.g. `2025-11-14T10:58:42-07:00_receiver.log`.
- Each file is JSON-lines; each line is a `PacketLog` serialized as JSON.
- Currently, logging is implemented for **clientbound** traffic (receiver side) and primarily for the play namespace (plus some earlier-phase packets as they occur).
- Direction and state are **not explicit fields**, but can usually be inferred from `Name`, e.g.:
  - `ClientboundLogin` (play join game)
  - `ClientboundDifficulty`
  - `ClientboundAbilities`
  - `ClientboundHeldItemSlot`

### Generated packet APIs (mc-protocol-go)

For each supported version (e.g. `1.21.5`) we generate:

- A versioned packet manager type, e.g. `data/1.21.5` package `v1_21_5.Packets`, exposed via:
  - `data/versions.GetPacketMgrForVersion(version string) models.PacketMgr`
- The `models.PacketMgr` interface includes:
  - Name / protocol metadata
  - Name→ID mapping by state + direction
  - ID→name mapping
  - ID→packet factory methods returning `models.PacketMarshaller`:
    - `GetClientboundLoginPacketByID(id models.ClientboundPacketID)`
    - `GetClientboundConfigPacketByID(id models.ClientboundPacketID)`
    - `GetClientboundPacketByID(id models.ClientboundPacketID)` (play)
    - `GetServerboundLoginPacketByID(id models.ServerboundPacketID)`
    - `GetServerboundConfigPacketByID(id models.ServerboundPacketID)`
    - `GetServerboundPacketByID(id models.ServerboundPacketID)` (play)

The `PacketMarshaller` interface is:

```go
type PacketMarshaller interface {
    Marshal() pk.Packet
    Scan(packet pk.Packet) error

    PacketID() int32

    GetFields() map[string]pk.FieldEncoder
    SetFields(fields map[string]pk.FieldEncoder)
}
```

So given a `PacketLog` and the right `PacketMgr`, we can:

1. Construct a `PacketMarshaller` instance from the packet ID.
2. Build a `pk.Packet` from the logged ID + data.
3. Call `Scan` to populate the struct.
4. Call `Marshal` to get a new `pk.Packet`.
5. Compare the original vs marshalled `Data` for a round-trip check.

---

## Target Architecture

### 1. Log generation (mc-agent)

- `mc-agent` is the **producer** of `PacketLog` entries.
- It should log:
  - **Direction** (clientbound / serverbound) encoded via the `Name` prefix (`Clientbound*` / `Serverbound*`).
  - **State/namespace** via the rest of the `Name` (e.g. `Login`, `Config`, implicit play).
  - `Version` and `ProtocolVersion` matching the concrete `PacketMgr` used in `mc-protocol-go`.
- Files are separated by role (already true for `*_receiver.log`). We can keep this convention for sender logs.

### 2. Log consumption + packet replay (mc-protocol-go)

Introduce a reusable **packet-log replay library** (internal to this repo):

- Responsible for:
  - Reading JSON-line logs into `models.PacketLog` structs (streaming or batched).
  - Choosing the correct `models.PacketMgr` based on `PacketLog.Version`.
  - Determining the **state + direction** using `PacketLog.Name` and the manager’s name→ID mappings.
  - Creating the appropriate `PacketMarshaller` with `Get*PacketByID`.
  - Building a `pk.Packet` from logged ID + data and calling `Scan`.
  - Calling `Marshal` and comparing the resulting `Data` with the original.
- Exposed via an API like:
  - `RunRoundTripChecks(ctx, source PacketLogSource, opts Options) (Summary, error)`

### 3. Test surfaces

- **Library-level tests**:
  - Unit tests that use synthetic `PacketLog` entries to validate routing and error handling.
- **Integration tests with real logs**:
  - Tests that read captured logs (small curated subsets) from `testing/` and validate real packets end-to-end.
  - Larger regression-style runs via a CLI tool or a `go test` flag + environment variable to point at a directory of logs.

---

## Implementation Plan

### Phase 1: Core packet-log replay library (mc-protocol-go) ✅ COMPLETE

**Implementation status:** ✅ **COMPLETE**

- ✅ `internal/packetlogtest/packetlogtest.go` with complete implementation
- ✅ `PacketLogSource` interface with `SliceSource` and `FileSource` implementations
- ✅ `RoundTripOptions`, `PacketError`, `Summary` types
- ✅ `RunRoundTripChecks` main driver with filtering, error collection, and context support
- ✅ Clientbound packet resolution via name→ID mapping across login/config/play states
- ✅ `Scan`/`Marshal` and `ReadFrom`/`WriteTo` validation for all generated packet types
- ✅ Panic handling for unknown versions and graceful error recovery
- ✅ `FormatSummary` helper for human-readable test output

#### 1.1 Create an internal package for replay logic

- Create `internal/packetlogtest` (or similar) with the following responsibilities:
  - Reading JSON-line files into `models.PacketLog`.
  - Resolving `PacketMgr` based on `Version`.
  - Determining state + direction from `Name`.
  - Performing the `Scan`/`Marshal` and `ReadFrom`/`WriteTo` checks.

Proposed public API (internal to this repo):

```go
// PacketLogSource abstracts where logs come from (files, memory, etc.).
type PacketLogSource interface {
    Next() (*models.PacketLog, error) // io.EOF when done
    Close() error
}

type RoundTripOptions struct {
    // Filter options
    Versions        []string
    MinTime, MaxTime *time.Time
    IncludeNames    []string // exact or prefix match
    ExcludeNames    []string

    // Behaviour
    StopOnFirstError bool
    MaxPackets       int // 0 = no limit
}

type PacketError struct {
    Log   *models.PacketLog
    Stage string // "resolve", "scan", "marshal", "compare", etc.
    Err   error
}

type Summary struct {
    Total       int
    Skipped     int
    Succeeded   int
    Failed      int
    ByVersion   map[string]int
    ByDirection map[string]int
    ByName      map[string]int
    Errors      []PacketError
}

func RunRoundTripChecks(ctx context.Context, src PacketLogSource, opts RoundTripOptions) (Summary, error)
```

#### 1.2 Implement `PacketLogSource` for JSON-line files

- Add a `FileSource` implementation in `internal/packetlogtest`:
  - Accepts a list of paths or an `io.Reader`.
  - Uses `json.Decoder` to stream-decode into `models.PacketLog`.
  - Handles large files without loading them fully into memory.

#### 1.3 Resolve `PacketMgr` from `PacketLog`

- Use the existing `data/versions.GetPacketMgrForVersion(version string)` helper:

```go
mgr := versions.GetPacketMgrForVersion(log.Version)
```

- Optionally validate that `mgr.VersionProtocol()` matches `log.ProtocolVersion` and record a diagnostic if not.

#### 1.4 Determine state + direction from `PacketLog.Name`

- Direction:
  - Use name prefix: `Clientbound*` vs `Serverbound*`.
- State/namespace:
  - Use the appropriate `Get*PacketID(name string)` methods on `PacketMgr`:
    - `GetClientboundLoginPacketID`, `GetClientboundConfigPacketID`, `GetClientboundPacketID` (play)
    - `GetServerboundLoginPacketID`, `GetServerboundConfigPacketID`, `GetServerboundPacketID` (play)
  - These functions currently **panic** on unknown names, so wrap them with `recover` to turn that into an error.
- Algorithm for clientbound (similar for serverbound once logs exist):
  1. Try each of the clientbound name→ID methods in order: Login, Config, Play.
  2. For each method, if it returns without panic, check whether the returned ID equals `PacketLog.ID`.
  3. If there’s a match, we have both the **state** and the canonical `ClientboundPacketID`.
  4. If none match, record a `PacketError{Stage: "resolve"}` and either skip or fail (depending on `opts.StopOnFirstError`).

This avoids guessing the state solely from string patterns and stays aligned with the generated mappings.

#### 1.5 Instantiate the correct packet struct

- Once we know direction + state and have a typed ID:
  - Clientbound:
    - Login: `mgr.GetClientboundLoginPacketByID(id)`
    - Config: `mgr.GetClientboundConfigPacketByID(id)`
    - Play: `mgr.GetClientboundPacketByID(id)`
  - Serverbound (future):
    - Login: `mgr.GetServerboundLoginPacketByID(id)`
    - Config: `mgr.GetServerboundConfigPacketByID(id)`
    - Play: `mgr.GetServerboundPacketByID(id)`

- Each returns a `models.PacketMarshaller` or an error. On error, record a `PacketError{Stage: "factory"}`.

#### 1.6 Build a `pk.Packet` and run `Scan` / `Marshal`

For each resolved packet:

1. Construct a `pk.Packet` with the logged ID + raw data:
   - `wire := pk.Packet{ID: pk.VarInt(log.ID), Data: log.Data}`
2. Call `packet.Scan(wire)` and capture any error.
3. Call `roundTrip := packet.Marshal()`.
4. Compare:
   - `roundTrip.ID` vs `wire.ID` (should match `packet.PacketID()`).
   - `roundTrip.Data` vs `wire.Data` (byte-for-byte equality for now).

Any mismatch or error results in a `PacketError{Stage: "scan"|"marshal"|"compare"}`.

#### 1.7 Validate `ReadFrom` / `WriteTo` using the same data

Where the generated struct also implements `io.ReaderFrom` / `io.WriterTo` (which is the case for generated packet structs), we can:

1. After a successful `Scan`, construct a fresh instance of the same type via the same factory.
2. Wrap `wire.Data` in a `bytes.Reader` and call `ReadFrom`:
   - Verify `ReadFrom` consumes all bytes (no leftover).
3. Call `WriteTo` into a new `bytes.Buffer`:
   - Verify the number of bytes written matches `len(wire.Data)`.
   - Verify the bytes match `wire.Data`.

This gives a second independent validation path for the generated `ReadFrom`/`WriteTo`, not just `Scan`/`Marshal`.

#### 1.8 Summarization and reporting

- As `RunRoundTripChecks` processes packets, accumulate:
  - Per-version counts.
  - Per-direction counts.
  - Per-packet name counts.
  - The first N `PacketError` instances (configurable cap to avoid memory blowup).
- Return a `Summary` struct and optionally a combined error when failures occur.

---

### Phase 2: Standalone CLI tool for packet log validation ✅ COMPLETE

**Implementation status:** ✅ **COMPLETE**

- ✅ Comprehensive unit test suite in `internal/packetlogtest/packetlogtest_test.go` (6 tests):
  - `TestRunRoundTripChecks_SingleClientbound_smoke`: validates basic round-trip for a synthetic packet
  - `TestRunRoundTripChecks_UnknownVersion`: error handling for unsupported versions
  - `TestRunRoundTripChecks_UnknownDirection`: error handling for malformed packet names
  - `TestRunRoundTripChecks_UnknownPacketName`: error handling for non-existent packets
  - `TestRunRoundTripChecks_FilterByVersion`: validates version filtering
  - `TestRunRoundTripChecks_MaxPackets`: validates packet count limiting
- ✅ Standalone CLI tool in `cmd/parse-packetlog/main.go` (296 lines):
  - Accepts files or directories as arguments, processes recursively
  - Rich filtering: `--versions`, `--include`, `--exclude`, `--max-packets`, `--stop-on-first`
  - Multiple output modes: text (default), `--summary` (stats only), `--json` (machine-readable)
  - Aggregates results across multiple files with overall summary
  - Exit code 1 if any packets fail validation (CI-friendly)
  - Complete usage documentation in `cmd/parse-packetlog/README.md`
- ✅ Test fixtures checked into `testing/packetlogs/1.21.5/sample_clientbound.log` with 10 diverse packets
- ✅ Documentation in `testing/packetlogs/README.md` explaining usage and known issues
- ✅ Successfully validates real packet logs from mc-agent (236 packets: 204 succeeded, 32 failed)
- ✅ Identifies multiple generator bugs: EOF errors, marshal mismatches, panics, NBT issues, metadata errors

This phase focuses on **using existing clientbound logs** to exercise the framework. Initially, we can restrict ourselves to one or two versions (e.g. 1.21.5) to keep iteration fast.

#### 2.1 Decide where test fixtures live

Options:

1. **Checked-in sample logs (recommended for fast CI):**
   - Copy a small subset of representative logs from `mc-agent/daze/logs` into this repo, e.g.:
     - `testing/packetlogs/1.21.5/clientbound_play.log`
   - Ensure they contain packets from a variety of states where available.
2. **External logs (for heavier, optional runs):**
   - Tests optionally read from an environment variable pointing at a log directory (e.g. `MC_PACKETLOG_DIR`).
   - If the env var is not set, tests are skipped or run only against the small checked-in fixtures.

Plan:

- Use (1) as the default so `go test` runs deterministically.
- Support (2) via an additional test or CLI for high-volume regression.

#### 2.2 Create test helpers under `testing/`

- Add a new test file, e.g. `testing/packetlog_roundtrip_test.go`.
- The test should:
  1. Discover log files under `testing/packetlogs/**`.
  2. For each log file:
     - Construct a `FileSource`.
     - Call `RunRoundTripChecks` with sensible `RoundTripOptions` (e.g., limit to a few thousand packets to keep tests fast).
     - Fail the test if there are any `Failed > 0`.
  3. Optionally, write per-packet-type counts to the test log for debugging.

Example test flow (conceptual):

```go
tfunc TestClientboundPacketLogs_RoundTrip(t *testing.T) {
    ctx := context.Background()

    logs := []string{
        "testing/packetlogs/1.21.5/clientbound_play.log",
        // more fixtures as they’re added
    }

    for _, path := range logs {
        t.Run(filepath.Base(path), func(t *testing.T) {
            src, err := packetlogtest.NewFileSource(path)
            require.NoError(t, err)
            defer src.Close()

            summary, err := packetlogtest.RunRoundTripChecks(ctx, src, packetlogtest.RoundTripOptions{
                StopOnFirstError: true,
                MaxPackets:       5000,
            })
            require.NoError(t, err)

            if summary.Failed > 0 {
                t.Fatalf("%s: %d packet(s) failed round-trip", path, summary.Failed)
            }
        })
    }
}
```

#### 2.3 Handle known limitations gracefully

- Some packets may not yet have fully wired `GetFields` / `SetFields` or may rely on behaviours that make strict byte-for-byte equality fragile.
- For the initial implementation:
  - Log but **don’t immediately fail** on a small number of known exceptions.
  - Alternatively, support a `KnownIssues` config that lists specific `(version, name)` pairs to skip.
- Use the integration tests to iteratively drive generator fixes until the exception list shrinks to zero or near-zero.

---

### Phase 3: Extend mc-agent logging to serverbound and all namespaces

This phase is primarily in the **mc-agent** repo, but the design lives here to keep the picture coherent.

#### 3.1 Identify all packet directions and states in mc-agent

- Locate the existing clientbound logging hook (receiver side) that emits `PacketLog`:
  - Understand whether it sees:
    - Handshaking
    - Status
    - Login
    - Configuration
    - Play
- Identify the corresponding serverbound paths (sender side) where we can log outgoing packets.

#### 3.2 Generalize logging into a shared helper

- Refactor mc-agent’s logging into a reusable helper function, e.g.:

```go
func logPacket(direction, name string, id int32, version string, protocolVersion uint, data []byte) {
    // Construct models.PacketLog and JSON-encode as a single line
}
```

- Use this helper in both clientbound (receiver) and serverbound (sender) code paths.

#### 3.3 Ensure `Name` values align with mc-protocol-go

- For each packet, choose a `Name` string that matches what `Get*PacketID(name string)` expects.
  - For play clientbound packets, the existing names like `ClientboundLogin`, `ClientboundDifficulty` already map via `GetClientboundPacketID`.
  - For login/config, use the names recognized by `GetClientboundLoginPacketID` / `GetClientboundConfigPacketID`.
  - For serverbound packets, use the `Serverbound*` names expected by the corresponding `GetServerbound*PacketID` methods.
- If necessary, add a thin helper in `mc-protocol-go` (or reuse existing maps) to avoid drifting name strings between repos.

#### 3.4 Add serverbound logs

- For each serverbound send path in mc-agent, call `logPacket`:
  - Direction: `"Serverbound"` (prefix the `Name` accordingly).
  - ID: the protocol packet ID as known by mc-protocol-go.
  - Data: the raw wire-format packet body.
- Store logs in mirrored files, e.g.:
  - `*_sender.log` for serverbound
  - `*_receiver.log` for clientbound (existing)

#### 3.5 Add logging for all namespaces

- Ensure logging is active during all connection phases:
  - Handshaking
  - Status
  - Login
  - Configuration
  - Play
- Validate by manually capturing a full session and checking that logs contain plausible packet names and IDs for each phase.

---

### Phase 4: Expand the test framework to serverbound and all namespaces

Once serverbound and non-play logs exist, expand the replay library and tests.

#### 4.1 Extend resolution logic for serverbound

- In `internal/packetlogtest`, add serverbound routing:
  - Direction detection from `Name` prefix.
  - Try `GetServerboundLoginPacketID`, `GetServerboundConfigPacketID`, `GetServerboundPacketID` as in the clientbound case.
  - Instantiate structs via the corresponding `GetServerbound*PacketByID` methods.

#### 4.2 Update integration tests to include serverbound logs

- Extend `testing/packetlogs` layout, e.g.:

```
testing/packetlogs/
  1.21.5/
    clientbound_play.log
    serverbound_play.log
    clientbound_login.log
    serverbound_login.log
    ...
```

- Add new tests / subtests:
  - `TestServerboundPacketLogs_RoundTrip`
  - `TestLoginPacketLogs_RoundTrip`
  - `TestConfigPacketLogs_RoundTrip`

#### 4.3 Tighten comparison rules

- After the framework is stable, enforce strictness:
  - Fail on any mismatch in `ID` or `Data`.
  - Fail on any `ReadFrom` not consuming exactly all bytes.
  - Consider verifying that `PacketID()` equals `PacketLog.ID` for additional safety.

#### 4.4 Track coverage per packet type

- Extend `Summary` to track coverage by packet type and state:
  - e.g. `coverage[version][direction][state][name] = count`
- Periodically generate a report (or log output) of which packet types have never been observed in logs.
- Optionally, add a small tool to output this as markdown/CSV for manual review.

---

### Phase 5: Tooling and CI integration

#### 5.1 CLI tool for ad-hoc verification ✅ COMPLETE

- ✅ Command `cmd/parse-packetlog` that:
  - Accepts paths to files or directories containing `*.log` files
  - Streams them through `RunRoundTripChecks` with configurable options
  - Writes human-readable summary to stdout with detailed error reporting
  - Supports JSON output mode (`--json`) for machine-readable reports
  - Allows filtering by version, packet name patterns
  - Provides `--summary` mode for quick stats without error details
- This allows quick testing against new logs without running `go test`.

#### 5.2 CI wiring (PENDING)

- Add a CI job that runs the `parse-packetlog` tool against the **checked-in** fixtures:

```bash
parse-packetlog testing/packetlogs/
```

- Keep the fixtures small to avoid slowing down CI.
- Optionally add a non-default CI job that runs against a larger, external log corpus if available.

#### 5.3 Developer workflow ✅ COMPLETE

- Document a simple workflow:
  1. Run mc-agent against a specific MC version with packet logging enabled.
  2. Run `parse-packetlog /path/to/mc-agent/logs` to validate all logs.
  3. Use filtering options to focus on specific issues:
     - `parse-packetlog --include ClientboundEntity logs/` for entity packets
     - `parse-packetlog --exclude ClientboundDeclareRecipes logs/` to skip known issues
     - `parse-packetlog --summary logs/` for quick overview
  4. Explore failures and fix generator issues.
  5. Optionally copy curated samples to `testing/packetlogs/<version>/` for regression testing.

---

## Edge Cases & Considerations

1. **Name / ID mismatches**
   - If mc-agent logs a `Name` that doesn’t exist in mc-protocol-go’s mappings, resolution will fail at the name→ID step.
   - The framework should clearly report these with the raw `Name`, `ID`, and `Version` so mappings can be fixed.

2. **Unsupported versions**
   - If a log contains `Version` values that don’t exist in `data/versions.GetPacketMgrForVersion`, the framework should:
     - Either skip with a clear warning, or
     - Fail fast depending on configuration.

3. **Protocol evolution**
   - As Mojang changes packets between versions, older logs may become incompatible with newer generated code.
   - The framework should always map using the **logged** `Version`, not the latest.

4. **Performance**
   - Large logs can be expensive to process.
   - Streaming decoding + `MaxPackets` in `RoundTripOptions` keeps tests fast while still sampling a diverse set of packets.

5. **Non-deterministic fields**
   - Some packets may contain timestamps, random IDs, or other fields where strict equality is tricky.
   - In most cases, we’re validating that **encoding/decoding is consistent**, not that values are stable.
   - If any packet types exhibit non-deterministic serialization, add targeted exceptions or additional comparison logic.

6. **ReadFrom / WriteTo symmetry**
   - Not all generated types may strictly conform to the expected `ReadFrom` / `WriteTo` patterns initially.
   - The framework should make it easy to identify where symmetry fails so generator fixes can be targeted.

---

## Success Criteria

### Phase 1 & 2 (Complete)

- [x] ✅ For at least one version (1.21.5), a packet-log-based validation runs over clientbound play packets
- [x] ✅ Framework successfully identifies multiple issues in generated code (32+ errors in real logs)
- [x] ✅ The core library test suite is fast (<1s for unit tests)
- [x] ✅ Failures clearly indicate `(version, direction, state, name, id)` and error stage
- [x] ✅ Standalone CLI tool provides flexible validation with filtering and output options
- [x] ✅ Core library has comprehensive error-path coverage (6 unit tests)
- [x] ✅ CLI tool supports files, directories, recursive search, and result aggregation
- [x] ✅ Documentation complete: `cmd/parse-packetlog/README.md` with usage examples

### Phase 3-5 (Pending)

- [ ] Serverbound logs are captured by mc-agent and consumed by the same framework
- [ ] All namespaces (handshaking, status, login, configuration, play) are exercised where logs exist
- [ ] Known real-world packets all round-trip successfully (currently 60% success rate for fixtures)

---

## Future Enhancements

1. **Fuzzing based on real packets**
   - Use real packets as seeds for go fuzz tests, mutating fields while ensuring encoding/decoding invariants hold.

2. **Cross-version compatibility checks**
   - For packets that are stable across versions, test round-tripping with different `PacketMgr` implementations to detect unintended changes.

3. **Automatic corpus generation**
   - Periodically regenerate a fresh packet log corpus by driving a bot through scripted scenarios and committing small, curated subsets as fixtures.

4. **Per-packet statistical analysis**
   - Use the summary data to detect rarely-seen packets and design targeted scenarios in mc-agent to capture them.
