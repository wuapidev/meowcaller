package meowcaller

import (
	"net"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/purpshell/meowcaller/diag"
	"github.com/rs/zerolog"
)

// Option configures optional, non-behavioral aspects of the call/media types —
// currently the diagnostic logger. The zero configuration logs nothing.
type Option func(*config)

type config struct {
	log  zerolog.Logger
	diag *diag.Recorder

	mediaPacketConn func(relayAddr *net.UDPAddr) (net.PacketConn, error)
	audioRTP        func(callID string, ssrc uint32, seq uint16, timestamp uint32)
	reorderWindow   time.Duration
	reorderReport   func(callID string, audio, video AudioReorderReport)
	offerGate       func(Offer) bool
}

func resolveConfig(opts []Option) config {
	c := config{log: zerolog.Nop(), reorderWindow: DefaultAudioReorderWindow}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// WithLogger sets the zerolog logger for debug/trace diagnostics. The library never
// configures logging itself; without this option the types are silent at zero cost.
// Pass the logger from a context, e.g. WithLogger(*zerolog.Ctx(ctx)).
func WithLogger(l zerolog.Logger) Option {
	return func(c *config) { c.log = l }
}

// WithDiagnostics attaches a developer-only *diag.Recorder that dumps exact,
// per-category call diagnostics (including raw secrets and media) to JSONL files.
// This is an opt-in maintainer carve-out from the library's sanitized logging and
// must never be enabled in production. Without it the recorder is nil and every
// diag emit is a no-op at zero cost.
func WithDiagnostics(rec *diag.Recorder) Option {
	return func(c *config) { c.diag = rec }
}

// WithMediaPacketConn makes every relay media connection run over the datagram
// socket fn returns instead of a plain host UDP socket, so media can leave through
// the same exit as signalling (a SOCKS5 UDP association).
func WithMediaPacketConn(fn func(relayAddr *net.UDPAddr) (net.PacketConn, error)) Option {
	return func(c *config) { c.mediaPacketConn = fn }
}

// WithAudioRTPObserver calls fn for every audio RTP packet a call receives from the
// relay, in arrival order, before the reorder stage and before decryption, so a consumer can measure loss,
// duplicates and reordering on the media path. fn runs on the receive loop: keep it short.
func WithAudioRTPObserver(fn func(callID string, ssrc uint32, seq uint16, timestamp uint32)) Option {
	return func(c *config) { c.audioRTP = fn }
}

// WithAudioReorderWindow sets how long inbound audio or video that arrived ahead of a missing
// packet waits for it (DefaultAudioReorderWindow when unset). Zero or less turns the
// reorder stage off: packets reach the decoder in arrival order.
func WithAudioReorderWindow(window time.Duration) Option {
	return func(c *config) { c.reorderWindow = window }
}

// WithAudioReorderReport calls fn once per call, when its media ends, with what the
// reorder stage did to the call's inbound audio and video.
func WithAudioReorderReport(fn func(callID string, audio, video AudioReorderReport)) Option {
	return func(c *config) { c.reorderReport = fn }
}

// Offer is what an offer gate is told about an inbound call before the library
// touches it.
type Offer struct {
	CallID string
	// From is the stanza's sender; Creator is the call-creator, the same user on a
	// 1:1 call.
	From    types.JID
	Creator types.JID
	// Group reports a group call or call-link invite.
	Group bool
	// Video reports that the offer advertises video.
	Video bool
}

// WithOfferGate makes the library ask before it engages with an inbound offer. When
// fn returns false nothing is decrypted, no <preaccept> is sent, no media starts and
// OnIncomingCall does not fire: the device stays as passive as a plain whatsmeow
// client. Without a gate every offer is engaged. fn runs on whatsmeow's event
// dispatch: keep it short.
func WithOfferGate(fn func(Offer) bool) Option {
	return func(c *config) { c.offerGate = fn }
}
