// Copyright 2015 The CRG Authors (see AUTHORS file).
// All rights reserved.  Use of this source code is
// governed by a GPL-style license that can be found
// in the LICENSE file.

package scoreboard

import (
	"log"

	"github.com/rollerderby/crg/state"
)

// Scoreboard is the realtime scoreboard controller.  All aspects of the
// live state for the scoreboard are contained within and exported
// via github.com/rollerderby/crg/state
type Scoreboard struct {
	stateIDs       map[string]string
	teams          []*team
	masterClock    *masterClock
	state          string
	snapshots      []*stateSnapshot
	jams           []*jam
	activeSnapshot *stateSnapshot
	activeJam      *jam
}

const (
	stateNotRunning   = ""
	statePreGame      = "PreGame"
	stateJam          = "Jam"
	stateLineup       = "Lineup"
	stateOTO          = "OTO"
	stateTTO1         = "TTO1"
	stateTTO2         = "TTO2"
	stateOR1          = "OR1"
	stateOR2          = "OR2"
	stateIntermission = "Intermission"
	stateUnofficial   = "UnofficialFinal"
	stateFinal        = "Final"
)

type parent interface {
	stateBase() string
}

// New initialized a default state for the scoreboard.  Additional setup
// of the scoreboard is required from either a saved state or via
// the web interface.  Returns a *Scoreboard
func New() *Scoreboard {
	sb := &Scoreboard{}
	sb.teams = append(sb.teams, newTeam(sb, 1), newTeam(sb, 2))
	sb.masterClock = newMasterClock(sb)

	sb.stateIDs = make(map[string]string)
	sb.stateIDs["state"] = sb.stateBase() + ".State"

	state.RegisterUpdaterString(sb.stateIDs["state"], 0, sb.setState)

	state.RegisterCommand("Scoreboard.StartJam", sb.startJam)
	state.RegisterCommand("Scoreboard.StopJam", sb.stopJam)
	state.RegisterCommand("Scoreboard.Timeout", sb.timeout)
	state.RegisterCommand("Scoreboard.EndTimeout", sb.endTimeout)
	state.RegisterCommand("Scoreboard.Undo", sb.undo)

	state.RegisterCommand("Scoreboard.Reset", sb.reset)

	// Setup Updaters for stateSnapshots (functions located in state_snapshot.go)
	state.RegisterPatternUpdaterString(sb.stateBase()+".Snapshot(*).State", 0, sb.ssSetState)
	state.RegisterPatternUpdaterBool(sb.stateBase()+".Snapshot(*).InProgress", 0, sb.ssSetInProgress)
	state.RegisterPatternUpdaterBool(sb.stateBase()+".Snapshot(*).CanRevert", 0, sb.ssSetCanRevert)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).StartTicks", 0, sb.ssSetStartTicks)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).EndTicks", 0, sb.ssSetEndTicks)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Length", 0, sb.ssSetLength)
	state.RegisterPatternUpdaterTime(sb.stateBase()+".Snapshot(*).StartTime", 0, sb.ssSetStartTime)
	state.RegisterPatternUpdaterTime(sb.stateBase()+".Snapshot(*).EndTime", 0, sb.ssSetEndTime)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Clock(*).Number", 0, sb.sscSetNumber)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Clock(*).StartTime", 0, sb.sscSetStartTime)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Clock(*).EndTime", 0, sb.sscSetEndTime)
	state.RegisterPatternUpdaterBool(sb.stateBase()+".Snapshot(*).Clock(*).Running", 0, sb.sscSetRunning)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Team(*).Timeouts", 0, sb.sstSetTimeouts)
	state.RegisterPatternUpdaterInt64(sb.stateBase()+".Snapshot(*).Team(*).OfficialReviews", 0, sb.sstSetOfficialReviews)
	state.RegisterPatternUpdaterBool(sb.stateBase()+".Snapshot(*).Team(*).OfficialReviewRetained", 0, sb.sstSetOfficialReviewRetained)

	sb.reset(nil)

	return sb
}

func (sb *Scoreboard) reset(_ []string) error {
	sb.setState(stateNotRunning)
	for _, t := range sb.teams {
		t.reset()
	}
	sb.masterClock.reset()

	for _, ss := range sb.snapshots {
		ss.delete()
	}
	for _, j := range sb.jams {
		j.delete()
	}
	sb.snapshots = nil
	sb.jams = nil
	sb.activeSnapshot = nil
	sb.activeJam = nil
	sb.snapshotStateStart()

	newJam(sb)
	log.Printf("sb.jams: %+v %v", sb.jams, len(sb.jams))

	return nil
}

func (sb *Scoreboard) snapshotStateStart() {
	sb.activeSnapshot = newStateSnapshot(sb, int64(len(sb.snapshots)), sb.masterClock.CurrentTime())
	sb.snapshots = append(sb.snapshots, sb.activeSnapshot)
}

func (sb *Scoreboard) snapshotStateEnd(canUndo bool) {
	// Check for an active snapshot
	if sb.activeSnapshot == nil {
		return
	}

	sb.activeSnapshot.end(canUndo, sb.masterClock.CurrentTime())
}

func (sb *Scoreboard) clocksExpired() {
	switch sb.state {
	case stateLineup:
		if !sb.masterClock.period.running {
			// Period clock ended, go to intermission or unofficial
			sb.endOfPeriod(false)
		} else {
			// Lineup expired, start jam!
			sb.startJam(nil)
		}
	case stateJam:
		if !sb.masterClock.jam.running {
			if !sb.masterClock.period.running {
				// Period clock is out, go to intermission or unofficial
				sb.endOfPeriod(false)
				return
			}
			sb.stopJam(nil)
		}
	case stateIntermission:
		if sb.masterClock.intermission.number.num == 1 {
			sb.endOfIntermission()
		}
	}
}

func (sb *Scoreboard) endOfPeriod(canUndo bool) {
	sb.snapshotStateEnd(canUndo)
	defer sb.snapshotStateStart()
	if sb.masterClock.period.number.num == 1 {
		sb.setState(stateIntermission)

		// Reset & start intermission clock
		sb.masterClock.intermission.reset(false, false)
		sb.masterClock.setRunningClocks(clockIntermission)
	} else {
		sb.setState(stateUnofficial)
		sb.masterClock.setRunningClocks()
	}
}

func (sb *Scoreboard) endOfIntermission() {
	log.Printf("END OF INTERMISSION!")
	sb.masterClock.period.reset(false, true)
	sb.masterClock.jam.reset(true, false)
}

func (sb *Scoreboard) stateBase() string {
	return "Scoreboard"
}

func (sb *Scoreboard) setState(v string) error {
	log.Printf("scoreboard: setState(%+v)", v)
	sb.state = v
	state.StateUpdateString(sb.stateIDs["state"], v)

	adjustable := false
	if isTimeoutState(v) {
		adjustable = true
	}
	sb.masterClock.setClockAdjustable(clockPeriod, adjustable)
	return nil
}

func (sb *Scoreboard) startJam(_ []string) error {
	if sb.state == stateJam {
		return nil
	}

	sb.snapshotStateEnd(true)
	defer sb.snapshotStateStart()

	if sb.state == stateIntermission {
		if sb.masterClock.intermission.time.num < sb.masterClock.intermission.time.max/2 {
			sb.endOfIntermission()
		}
	}

	sb.setState(stateJam)

	// Reset jam clock and increment jam number
	sb.masterClock.jam.reset(false, true)
	// Start clocks Period, Jam
	sb.masterClock.setRunningClocks(clockPeriod, clockJam)
	sb.activeJam.updateJam()
	return nil
}

func (sb *Scoreboard) stopJam(_ []string) error {
	if sb.state != stateJam {
		return nil
	}

	if !sb.masterClock.period.running {
		// Period clock is out, go to intermission or unofficial
		sb.endOfPeriod(true)
		return nil
	}

	// Not the end of a period, start lineups
	sb.snapshotStateEnd(sb.masterClock.jam.time.num != sb.masterClock.jam.time.min)
	defer sb.snapshotStateStart()
	sb.setState(stateLineup)

	newJam(sb)

	// Reset lineup clock
	sb.masterClock.lineup.reset(false, false)
	// Start clocks Period, Lineup
	sb.masterClock.setRunningClocks(clockPeriod, clockLineup)
	return nil
}

func (sb *Scoreboard) timeout(data []string) error {
	var newState = stateOTO
	if len(data) > 0 {
		if isTimeoutState(data[0]) {
			newState = data[0]
		}
	}

	stateChanged := false
	sb.snapshotStateEnd(true)
	defer func() {
		if stateChanged {
			sb.snapshotStateStart()
		} else {
			if sb.activeSnapshot != nil {
				sb.activeSnapshot.unend()
			}
		}
	}()

	switch newState {
	case stateOTO:
		if sb.state == stateOTO {
			// Already in OTO
			return nil
		}
	case stateTTO1:
		if !sb.teams[0].useTimeout() {
			// Timeout not available
			return nil
		}
	case stateTTO2:
		if !sb.teams[1].useTimeout() {
			// Timeout not available
			return nil
		}
	case stateOR1:
		if !sb.teams[0].useOfficialReview() {
			// OfficialReview not available
			return nil
		}
	case stateOR2:
		if !sb.teams[1].useOfficialReview() {
			// OfficialReview not available
			return nil
		}
	}

	stateChanged = true
	sb.setState(newState)

	// Reset timeout clock
	sb.masterClock.timeout.reset(false, false)
	// Start clocks Timeout
	sb.masterClock.setRunningClocks(clockTimeout)
	return nil
}

func (sb *Scoreboard) endTimeout(_ []string) error {
	if !isTimeoutState(sb.state) {
		return nil
	}
	sb.snapshotStateEnd(true)
	defer sb.snapshotStateStart()
	sb.setState(stateLineup)

	// Reset timeout clock
	sb.masterClock.lineup.reset(false, false)
	// Start clocks Timeout
	sb.masterClock.setRunningClocks(clockLineup)
	return nil
}

func (sb *Scoreboard) undo(_ []string) error {
	if len(sb.snapshots) > 1 {
		lastSnapshot := sb.snapshots[len(sb.snapshots)-2]
		if !lastSnapshot.canRevert {
			return nil
		}
		log.Printf("Scoreboard.undo: REVERTING")

		if sb.state != stateJam && lastSnapshot.state == stateJam {
			sb.activeJam.delete()
			sb.activeJam = sb.jams[len(sb.jams)-2]
			sb.jams = sb.jams[:len(sb.jams)-2]
			sb.activeJam.reinstatePositions()
		}

		for name, c := range lastSnapshot.clocks {
			clock := sb.masterClock.clocks[name]
			clock.setRunning(c.running)
			clock.time.setNum(c.endTime)
			clock.number.setNum(c.number)
		}
		for t := 0; t < 2; t++ {
			sb.teams[t].setTimeouts(lastSnapshot.teams[0].timeouts)
			sb.teams[t].setOfficialReviews(lastSnapshot.teams[0].officialReviews)
			sb.teams[t].setOfficialReviewRetained(lastSnapshot.teams[0].officialReviewRetained)
		}
		sb.masterClock.setTicks(lastSnapshot.endTicks)

		sb.setState(lastSnapshot.state)

		sb.activeSnapshot.delete()
		lastSnapshot.unend()
		sb.activeSnapshot = lastSnapshot
		sb.snapshots[len(sb.snapshots)-1] = nil
		sb.snapshots = sb.snapshots[:len(sb.snapshots)-1]

		sb.masterClock.ticker()
	}
	return nil
}

func isTimeoutState(state string) bool {
	return state == stateOTO ||
		state == stateTTO1 ||
		state == stateTTO2 ||
		state == stateOR1 ||
		state == stateOR2
}
