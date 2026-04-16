//go:build !js

package serve

// Option is a functional option for [NewServer]. See NewServer for how
// options are sequenced and how additive vs. last-wins options compose.
type Option func(*Server)
