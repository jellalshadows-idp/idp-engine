// Package floci holds the level-3 tests of the AWS modules and the AWS stacks
// that render them (spec §8.4). CI's floci job renders the renderer's
// aws-registry fixture, adds floci_override.tf.json to each AWS stack, applies
// the stacks to two accounts of one floci emulator, replays them, checks the
// created identities and policies through the AWS API against internal/naming,
// and destroys them. The package has no code outside its tests, and TestFloci
// skips unless IDP_FLOCI_ENDPOINT is set, so go test ./... stays offline.
package floci
