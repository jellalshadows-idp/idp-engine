// Package floci holds the level-3 tests of the AWS modules (spec §8.4). CI's
// floci job applies modules/aws/baseline and modules/aws/component to two
// accounts of one floci emulator, with the inputs that internal/naming
// computes, replays them, checks the created identities and policies through
// the AWS API, and destroys them. The package has no code outside its tests,
// and TestFloci skips unless IDP_FLOCI_ENDPOINT is set, so go test ./... stays
// offline.
package floci
