package main

type ChangeStatus int

const (
	_ = iota

	CHNotApplied ChangeStatus = iota
	CHSucceeded
	CHFailed
	CHNoChangeRequested
)

type Verbosity int

const (
	VHigh Verbosity = iota
	VChangesOnly
	VOff
)

type ChownOption struct {
	verbosity Verbosity
	recurse   bool
	// rootDevIno *devIno
	affectSymlinkReferent bool
	forceSilent           bool
	userName              string
	groupName             string
}

type RCHStatus int

const (
	_ = iota
	_ = iota

	RCOK RCHStatus = iota
	RCExcluded
	RCInodeChanged
	RCDoOrdinaryChown
	RCError
)
