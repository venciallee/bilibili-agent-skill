// Package bilibiliskill ships the data files the bili CLI embeds.
//
// The hook formulas live next to the skill that teaches them
// (skills/bili-script/hooks.json) so a model reading the skill and the
// binary scoring a hook always agree. Embedding from the module root is
// what lets one file serve both.
package bilibiliskill

import _ "embed"

// HooksJSON is skills/bili-script/hooks.json, the 21 hook formulas.
//
//go:embed skills/bili-script/hooks.json
var HooksJSON []byte
