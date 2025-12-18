package packetlogtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/reallyoldfogie/mc-protocol-go/data/versions"
	"github.com/reallyoldfogie/mc-protocol-go/models"

	pk "github.com/Tnze/go-mc/net/packet"
)

// PacketLogSource abstracts where logs come from (files, memory, etc.).
// Implementations must be safe for sequential use from a single goroutine.
type PacketLogSource interface {
	Next() (*models.PacketLog, error) // returns io.EOF when exhausted
	Close() error
}

// SliceSource implements PacketLogSource for an in-memory slice of PacketLog entries.
// Useful for testing without file I/O.
type SliceSource struct {
	logs []*models.PacketLog
	idx  int
}

// NewSliceSource creates a SliceSource from a slice of PacketLog pointers.
func NewSliceSource(logs []*models.PacketLog) *SliceSource {
	return &SliceSource{logs: logs}
}

func (s *SliceSource) Next() (*models.PacketLog, error) {
	if s.idx >= len(s.logs) {
		return nil, io.EOF
	}
	pl := s.logs[s.idx]
	s.idx++
	return pl, nil
}

func (s *SliceSource) Close() error { return nil }

// FileSource implements PacketLogSource for one or more JSON-lines files
// containing PacketLog entries.
type FileSource struct {
	files []string

	curFile *os.File
	dec     *json.Decoder
	idx     int
}

// NewFileSource creates a FileSource from a list of log file paths.
// Files are read sequentially in the order provided.
func NewFileSource(paths ...string) (*FileSource, error) {
	if len(paths) == 0 {
		return nil, errors.New("packetlogtest: no paths provided")
	}
	return &FileSource{files: append([]string(nil), paths...)}, nil
}

func (s *FileSource) openNext() error {
	if s.idx >= len(s.files) {
		return io.EOF
	}

	if s.curFile != nil {
		_ = s.curFile.Close()
	}

	f, err := os.Open(s.files[s.idx])
	if err != nil {
		return fmt.Errorf("packetlogtest: open %s: %w", s.files[s.idx], err)
	}

	s.curFile = f
	s.dec = json.NewDecoder(f)
	s.idx++
	return nil
}

// Next returns the next PacketLog from the underlying files.
// It transparently advances to the next file when the current one is exhausted.
func (s *FileSource) Next() (*models.PacketLog, error) {
	for {
		if s.dec == nil {
			if err := s.openNext(); err != nil {
				return nil, err
			}
		}

		var logEntry models.PacketLog
		if err := s.dec.Decode(&logEntry); err != nil {
			if errors.Is(err, io.EOF) {
				// Move to next file
				s.dec = nil
				continue
			}
			return nil, fmt.Errorf("packetlogtest: decode packet log: %w", err)
		}
		return &logEntry, nil
	}
}

// Close closes the current file handle, if any.
func (s *FileSource) Close() error {
	if s.curFile != nil {
		return s.curFile.Close()
	}
	return nil
}

// RoundTripOptions controls how RunRoundTripChecks behaves.
// All fields are optional; zero values select reasonable defaults.
type RoundTripOptions struct {
	// Filter options
	Versions     []string
	MinTime      *time.Time
	MaxTime      *time.Time
	IncludeNames []string // exact or prefix match
	ExcludeNames []string

	// Behaviour
	StopOnFirstError bool
	MaxPackets       int // 0 = no limit

	// Error collection
	MaxErrors int // 0 = default (e.g. 100)
}

// PacketError describes a failure processing a single PacketLog.
// Stage indicates where it failed: resolve, scan, marshal, compare, readfrom, writeto, etc.
type PacketError struct {
	Log   *models.PacketLog
	Stage string
	Err   error
}

// ErrorSummary represents a unique error with occurrence count.
type ErrorSummary struct {
	Stage   string `json:"stage"`
	Name    string `json:"name"`
	ID      int32  `json:"id"`
	Version string `json:"version"`
	Error   string `json:"error"`
	Count   int    `json:"count"`
}

// Summary aggregates the results of a RunRoundTripChecks invocation.
type Summary struct {
	Total     int
	Skipped   int
	Succeeded int
	Failed    int

	ByVersion   map[string]int
	ByDirection map[string]int
	ByName      map[string]int

	// Deprecated: Use ErrorSummaries instead
	Errors []PacketError `json:"-"`
	// ErrorSummaries contains unique errors with occurrence counts
	ErrorSummaries []ErrorSummary `json:"errors"`
}

// FormatSummary returns a human-readable multi-line summary of results.
func FormatSummary(s Summary, includeErrors bool) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "Total: %d, Succeeded: %d, Failed: %d, Skipped: %d\n", s.Total, s.Succeeded, s.Failed, s.Skipped)

	if len(s.ByVersion) > 0 {
		fmt.Fprintf(&buf, "By Version:\n")
		for v, count := range s.ByVersion {
			fmt.Fprintf(&buf, "  %s: %d\n", v, count)
		}
	}

	if len(s.ByDirection) > 0 {
		fmt.Fprintf(&buf, "By Direction:\n")
		for dir, count := range s.ByDirection {
			fmt.Fprintf(&buf, "  %s: %d\n", dir, count)
		}
	}

	if len(s.ByName) > 0 {
		fmt.Fprintf(&buf, "By Packet Name (top 10):\n")
		type nameCount struct {
			name  string
			count int
		}
		var names []nameCount
		for name, count := range s.ByName {
			names = append(names, nameCount{name: name, count: count})
		}
		// Simple descending sort by count
		for i := range names {
			for j := i + 1; j < len(names); j++ {
				if names[j].count > names[i].count {
					names[i], names[j] = names[j], names[i]
				}
			}
		}
		limit := 10
		if len(names) < limit {
			limit = len(names)
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&buf, "  %s: %d\n", names[i].name, names[i].count)
		}
		if len(names) > 10 {
			fmt.Fprintf(&buf, "  ... and %d more\n", len(names)-10)
		}
	}

	if includeErrors && len(s.ErrorSummaries) > 0 {
		fmt.Fprintf(&buf, "Unique Errors (%d):\n", len(s.ErrorSummaries))
		for i, es := range s.ErrorSummaries {
			fmt.Fprintf(&buf, "  %d. [%s] %s (id=%d version=%s) - %d occurrence(s): %s\n",
				i+1, es.Stage, es.Name, es.ID, es.Version, es.Count, es.Error)
		}
	}

	return buf.String()
}

// RunRoundTripChecks streams PacketLog entries from src, resolves the
// corresponding generated packet types and performs Scan/Marshal and
// ReadFrom/WriteTo round-trip validation where possible.
func RunRoundTripChecks(ctx context.Context, src PacketLogSource, opts RoundTripOptions) (Summary, error) {
	if opts.MaxErrors <= 0 {
		opts.MaxErrors = 100
	}

	summary := Summary{
		ByVersion:   map[string]int{},
		ByDirection: map[string]int{},
		ByName:      map[string]int{},
	}

	// Track unique errors with counts
	errorMap := make(map[string]*ErrorSummary)

	matchesFilter := func(pl *models.PacketLog) bool {
		if len(opts.Versions) > 0 && !containsString(opts.Versions, pl.Version) {
			return false
		}
		if opts.MinTime != nil && pl.Timestamp.Before(*opts.MinTime) {
			return false
		}
		if opts.MaxTime != nil && pl.Timestamp.After(*opts.MaxTime) {
			return false
		}
		if len(opts.IncludeNames) > 0 && !matchesAnyName(opts.IncludeNames, pl.Name) {
			return false
		}
		if len(opts.ExcludeNames) > 0 && matchesAnyName(opts.ExcludeNames, pl.Name) {
			return false
		}
		return true
	}

	for {
		select {
		case <-ctx.Done():
			finalizeErrorSummaries(&summary, errorMap)
			return summary, ctx.Err()
		default:
		}

		if opts.MaxPackets > 0 && summary.Total >= opts.MaxPackets {
			finalizeErrorSummaries(&summary, errorMap)
			return summary, nil
		}

		pl, err := src.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				finalizeErrorSummaries(&summary, errorMap)
				return summary, nil
			}
			// Source error is considered fatal
			finalizeErrorSummaries(&summary, errorMap)
			return summary, err
		}

		// Only process play-state packets for now
		if pl.State == "" || pl.State == "play" {
			summary.Total++

			if !matchesFilter(pl) {
				summary.Skipped++
				continue
			}

			direction := pl.Direction
			if direction == "" {
				direction = directionFromName(pl.Name)
			}

			if direction == "" {
				recordError(&summary, opts, PacketError{Log: pl, Stage: "direction", Err: fmt.Errorf("unknown direction for name %q", pl.Name)}, errorMap)
				if opts.StopOnFirstError {
					return summary, fmt.Errorf("packetlogtest: direction resolution failed")
				}
				continue
			}
			mgr, err := getPacketMgr(pl)
			if err != nil {
				recordError(&summary, opts, PacketError{Log: pl, Stage: "packetmgr", Err: err}, errorMap)
				if opts.StopOnFirstError {
					return summary, err
				}
				continue
			}

			if err := processPacket(pl, mgr, direction); err != nil {
				recordError(&summary, opts, PacketError{Log: pl, Stage: "roundtrip", Err: err}, errorMap)
				if opts.StopOnFirstError {
					return summary, err
				}
				continue
			}

			// Success
			summary.Succeeded++
			summary.ByVersion[pl.Version]++
			summary.ByDirection[direction]++
			summary.ByName[pl.Name]++
		}
	}
}

// finalizeErrorSummaries converts the error map to a sorted slice
func finalizeErrorSummaries(summary *Summary, errorMap map[string]*ErrorSummary) {
	// Convert error map to sorted slice
	summary.ErrorSummaries = make([]ErrorSummary, 0, len(errorMap))
	for _, es := range errorMap {
		summary.ErrorSummaries = append(summary.ErrorSummaries, *es)
	}
	// Sort by count (descending), then by name
	for i := range summary.ErrorSummaries {
		for j := i + 1; j < len(summary.ErrorSummaries); j++ {
			if summary.ErrorSummaries[j].Count > summary.ErrorSummaries[i].Count ||
				(summary.ErrorSummaries[j].Count == summary.ErrorSummaries[i].Count &&
					summary.ErrorSummaries[j].Name < summary.ErrorSummaries[i].Name) {
				summary.ErrorSummaries[i], summary.ErrorSummaries[j] = summary.ErrorSummaries[j], summary.ErrorSummaries[i]
			}
		}
	}
}

func recordError(summary *Summary, opts RoundTripOptions, pe PacketError, errorMap map[string]*ErrorSummary) {
	summary.Failed++

	// Create unique key for this error type
	key := fmt.Sprintf("%s|%s|%d|%s|%s", pe.Stage, pe.Log.Name, pe.Log.ID, pe.Log.Version, pe.Err.Error())

	if es, exists := errorMap[key]; exists {
		// Increment count for existing error
		es.Count++
	} else if len(errorMap) < opts.MaxErrors {
		// Add new unique error
		errorMap[key] = &ErrorSummary{
			Stage:   pe.Stage,
			Name:    pe.Log.Name,
			ID:      pe.Log.ID,
			Version: pe.Log.Version,
			Error:   pe.Err.Error(),
			Count:   1,
		}
	}

	// Keep deprecated Errors field for backwards compatibility (limited)
	if len(summary.Errors) < 10 {
		summary.Errors = append(summary.Errors, pe)
	}
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func matchesAnyName(patterns []string, name string) bool {
	for _, pat := range patterns {
		if pat == name || strings.HasPrefix(name, pat) {
			return true
		}
	}
	return false
}

func directionFromName(name string) string {
	switch {
	case strings.HasPrefix(name, "Clientbound"):
		return "clientbound"
	case strings.HasPrefix(name, "Serverbound"):
		return "serverbound"
	default:
		return ""
	}
}

func getPacketMgr(pl *models.PacketLog) (mgr models.PacketMgr, err error) {
	// For now we only support versions that have a concrete PacketMgr.
	// GetPacketMgrForVersion will panic on unknown versions, so guard it.
	defer func() {
		if r := recover(); r != nil {
			mgr = nil
			err = fmt.Errorf("packetlogtest: panic getting PacketMgr for version %q: %v", pl.Version, r)
		}
	}()

	mgr = versions.GetPacketMgrForVersion(pl.Version)
	if mgr == nil {
		return nil, fmt.Errorf("packetlogtest: no PacketMgr for version %q", pl.Version)
	}

	// Optional: validate protocol version when present
	if pl.ProtocolVersion != 0 && mgr.VersionProtocol() != uint64(pl.ProtocolVersion) {
		return nil, fmt.Errorf("packetlogtest: protocol version mismatch for %q: log=%d mgr=%d", pl.Version, pl.ProtocolVersion, mgr.VersionProtocol())
	}

	return mgr, nil
}

// processPacket performs the actual Scan/Marshal and ReadFrom/WriteTo
// validations for a single PacketLog.
func processPacket(pl *models.PacketLog, mgr models.PacketMgr, direction string) (err error) {
	// Recover from panics that occur during packet processing (e.g., interface conversion failures)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("packetlogtest: panic processing %s: %v", pl.Name, r)
		}
	}()

	// For now, handle only clientbound packets. Serverbound will be added later
	// once logging is available.
	switch direction {
	case "clientbound":
		return processClientbound(pl, mgr)
	case "serverbound":
		// TODO: implement when serverbound logging is available
		return fmt.Errorf("packetlogtest: serverbound processing not yet implemented")
	default:
		return fmt.Errorf("packetlogtest: unknown direction %q", direction)
	}
}

func processClientbound(pl *models.PacketLog, mgr models.PacketMgr) error {
	// First resolve the packet ID and state using the name-based helpers.
	id, state, err := resolveClientboundID(mgr, pl)
	if err != nil {
		return err
	}

	// Create the appropriate packet instance from the ID.
	var marshaller models.PacketMarshaller
	switch state {
	case "login":
		marshaller, err = mgr.GetClientboundLoginPacketByID(id)
	case "config":
		marshaller, err = mgr.GetClientboundConfigPacketByID(id)
	case "play":
		marshaller, err = mgr.GetClientboundPacketByID(id)
	default:
		return fmt.Errorf("packetlogtest: unknown clientbound state %q", state)
	}
	if err != nil {
		return fmt.Errorf("packetlogtest: factory for state=%s id=%d: %w", state, id, err)
	}

	// Build the wire-format packet from the logged data.
	// pk.Packet.ID is an int32, and PacketLog.ID is the raw packet ID as seen on the wire.
	wire := pk.Packet{ID: pl.ID, Data: append([]byte(nil), pl.Data...)}

	// Scan into the struct.
	if err := marshaller.Scan(wire); err != nil {
		return fmt.Errorf("packetlogtest: Scan failed for %s (state=%s id=%d): %w", pl.Name, state, id, err)
	}

	// Marshal back out and compare.
	roundTrip := marshaller.Marshal()
	if int32(roundTrip.ID) != pl.ID {
		return fmt.Errorf("packetlogtest: Marshal ID mismatch for %s (state=%s): log=%d marshal=%d", pl.Name, state, pl.ID, roundTrip.ID)
	}
	if !bytes.Equal(roundTrip.Data, pl.Data) {
		return fmt.Errorf("packetlogtest: Marshal data mismatch for %s (state=%s id=%d): len(log)=%d len(marshal)=%d", pl.Name, state, id, len(pl.Data), len(roundTrip.Data))
	}

	// Validate ReadFrom/WriteTo if supported.
	if rf, ok := any(marshaller).(io.ReaderFrom); ok {
		if err := validateReaderFrom(rf, pl); err != nil {
			return err
		}
	}
	if wf, ok := any(marshaller).(io.WriterTo); ok {
		if err := validateWriterTo(wf, pl); err != nil {
			return err
		}
	}

	return nil
}

func validateReaderFrom(rf io.ReaderFrom, pl *models.PacketLog) error {
	buf := bytes.NewReader(pl.Data)
	n, err := rf.ReadFrom(buf)
	if err != nil {
		return fmt.Errorf("packetlogtest: ReadFrom failed for %s: %w", pl.Name, err)
	}
	if remaining := buf.Len(); remaining != 0 {
		return fmt.Errorf("packetlogtest: ReadFrom did not consume all bytes for %s: remaining=%d read=%d", pl.Name, remaining, n)
	}
	return nil
}

func validateWriterTo(wf io.WriterTo, pl *models.PacketLog) error {
	var buf bytes.Buffer
	n, err := wf.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("packetlogtest: WriteTo failed for %s: %w", pl.Name, err)
	}
	if int(n) != len(buf.Bytes()) {
		return fmt.Errorf("packetlogtest: WriteTo byte count mismatch for %s: reported=%d actual=%d", pl.Name, n, len(buf.Bytes()))
	}
	if !bytes.Equal(buf.Bytes(), pl.Data) {
		return fmt.Errorf("packetlogtest: WriteTo data mismatch for %s: len(log)=%d len(written)=%d", pl.Name, len(pl.Data), len(buf.Bytes()))
	}
	return nil
}

// resolveClientboundID determines the clientbound packet ID and state
// (login/config/play) corresponding to the given PacketLog.
func resolveClientboundID(mgr models.PacketMgr, pl *models.PacketLog) (models.ClientboundPacketID, string, error) {
	type resolver struct {
		state string
		fn    func(string) models.ClientboundPacketID
	}

	resolvers := []resolver{
		{state: "login", fn: mgr.GetClientboundLoginPacketID},
		{state: "config", fn: mgr.GetClientboundConfigPacketID},
		{state: "play", fn: mgr.GetClientboundPacketID},
	}

	var lastErr error
	for _, r := range resolvers {
		id, ok, err := safeClientboundLookup(r.fn, pl.Name)
		if err != nil {
			lastErr = err
			continue
		}
		if !ok {
			continue
		}
		// Ensure the ID matches the logged ID.
		if int32(id) != pl.ID {
			lastErr = fmt.Errorf("packetlogtest: clientbound ID mismatch for %s in state %s: log=%d lookup=%d", pl.Name, r.state, pl.ID, id)
			continue
		}
		return id, r.state, nil
	}

	if lastErr != nil {
		return 0, "", lastErr
	}
	return 0, "", fmt.Errorf("packetlogtest: could not resolve clientbound ID for %s (id=%d)", pl.Name, pl.ID)
}

// safeClientboundLookup wraps a name→ID lookup that may panic on unknown names.
func safeClientboundLookup(fn func(string) models.ClientboundPacketID, name string) (id models.ClientboundPacketID, ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			id = 0
			if e, okAssert := r.(error); okAssert {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()

	id = fn(name)
	return id, true, nil
}
