package cli

// Exit codes shared by every command (ADR-0016).
const (
	exitOK       = 0 // done; nothing needs attention
	exitError    = 1 // the command could not do its job
	exitUsage    = 2 // the command line is wrong
	exitFindings = 3 // done, and the answer needs attention (invalid claims, drift)
)
