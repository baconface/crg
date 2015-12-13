// Copyright 2015 The CRG Authors (see AUTHORS file).
// All rights reserved.  Use of this source code is
// governed by a GPL-style license that can be found
// in the LICENSE file.

package scoreboard

import (
	"fmt"

	"github.com/rollerderby/crg/state"
)

const (
	leadLead = "Lead"
	leadNo   = "No"
	leadLost = "Lost"
)

type team struct {
	sb                     *Scoreboard
	base                   string
	id                     uint8
	name                   string
	color                  string
	score                  int64
	lastScore              int64
	timeouts               int64
	officialReviews        int64
	officialReviewRetained bool
	lead                   string
	starPass               bool
	jammer                 string
	pivot                  string
	settings               map[string]*setting
	skaters                map[string]*skater
	stateIDs               map[string]string
}

func newTeam(sb *Scoreboard, id uint8) *team {
	t := &team{
		sb:       sb,
		base:     fmt.Sprintf("%s.Team(%d)", sb.stateBase(), id),
		id:       id,
		settings: make(map[string]*setting),
		skaters:  make(map[string]*skater),
		stateIDs: make(map[string]string),
	}

	t.stateIDs["id"] = fmt.Sprintf("%s.ID", t.base)
	t.stateIDs["name"] = fmt.Sprintf("%s.Name", t.base)
	t.stateIDs["color"] = fmt.Sprintf("%s.Color", t.base)
	t.stateIDs["score"] = fmt.Sprintf("%s.Score", t.base)
	t.stateIDs["lastScore"] = fmt.Sprintf("%s.LastScore", t.base)
	t.stateIDs["jamScore"] = fmt.Sprintf("%s.JamScore", t.base)
	t.stateIDs["timeouts"] = fmt.Sprintf("%s.Timeouts", t.base)
	t.stateIDs["officialReviews"] = fmt.Sprintf("%s.OfficialReviews", t.base)
	t.stateIDs["officialReviewRetained"] = fmt.Sprintf("%s.OfficialReviewRetained", t.base)
	t.stateIDs["lead"] = fmt.Sprintf("%s.Lead", t.base)
	t.stateIDs["starPass"] = fmt.Sprintf("%s.StarPass", t.base)
	t.stateIDs["jammer"] = fmt.Sprintf("%s.Jammer.ID", t.base)
	t.stateIDs["jammerInBox"] = fmt.Sprintf("%s.Jammer.InBox", t.base)
	t.stateIDs["pivot"] = fmt.Sprintf("%s.Pivot.ID", t.base)
	t.stateIDs["pivotInBox"] = fmt.Sprintf("%s.Pivot.InBox", t.base)

	state.StateUpdateInt64(t.stateIDs["id"], int64(id))

	state.RegisterUpdaterString(t.stateIDs["name"], 0, t.setName)
	state.RegisterUpdaterString(t.stateIDs["color"], 0, t.setColor)
	state.RegisterUpdaterInt64(t.stateIDs["score"], 0, t.setScore)
	state.RegisterUpdaterInt64(t.stateIDs["lastScore"], 0, t.setLastScore)
	state.RegisterUpdaterInt64(t.stateIDs["timeouts"], 0, t.setTimeouts)
	state.RegisterUpdaterInt64(t.stateIDs["officialReviews"], 0, t.setOfficialReviews)
	state.RegisterUpdaterBool(t.stateIDs["officialReviewRetained"], 0, t.setOfficialReviewRetained)
	state.RegisterUpdaterString(t.stateIDs["lead"], 0, t.setLead)
	state.RegisterUpdaterBool(t.stateIDs["starPass"], 0, t.setStarPass)

	state.RegisterUpdaterString(t.stateIDs["jammer"], 1, t.setJammer)         // Must be after skaters are loaded
	state.RegisterUpdaterString(t.stateIDs["pivot"], 1, t.setPivot)           // Must be after skaters are loaded
	state.RegisterUpdaterBool(t.stateIDs["jammerInBox"], 1, t.setJammerInBox) // Must be after skaters are loaded
	state.RegisterUpdaterBool(t.stateIDs["pivotInBox"], 1, t.setPivotInBox)   // Must be after skaters are loaded

	state.RegisterCommand(t.stateIDs["score"]+".Inc", t.incScore)
	state.RegisterCommand(t.stateIDs["score"]+".Dec", t.decScore)
	state.RegisterCommand(t.stateIDs["lastScore"]+".Inc", t.incLastScore)
	state.RegisterCommand(t.stateIDs["lastScore"]+".Dec", t.decLastScore)
	state.RegisterCommand(t.stateIDs["timeouts"]+".Start", t.startTimeout)
	state.RegisterCommand(t.stateIDs["officialReviews"]+".Start", t.startOfficialReview)
	state.RegisterCommand(t.stateIDs["officialReviews"]+".Retained", t.retainOfficialReview)
	state.RegisterCommand(t.base+".DeleteSkater", t.deleteSkater)

	// Setup Updaters for skaters (functions located in skater.go)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).ID", 0, t.sSetID)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).Name", 0, t.sSetName)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).LegalName", 0, t.sSetLegalName)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).InsuranceNumber", 0, t.sSetInsuranceNumber)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).Number", 0, t.sSetNumber)
	state.RegisterPatternUpdaterString(t.base+".Skater(*).Position", 0, t.sSetPosition)
	state.RegisterPatternUpdaterBool(t.base+".Skater(*).IsAlt", 0, t.sSetIsAlt)
	state.RegisterPatternUpdaterBool(t.base+".Skater(*).IsCaptain", 0, t.sSetIsCaptain)
	state.RegisterPatternUpdaterBool(t.base+".Skater(*).IsAltCaptain", 0, t.sSetIsAltCaptain)
	state.RegisterPatternUpdaterBool(t.base+".Skater(*).IsBenchStaff", 0, t.sSetIsBenchStaff)
	state.RegisterPatternUpdaterBool(t.base+".Skater(*).InBox", 0, t.sSetInBox)

	t.reset()
	return t
}

func (t *team) reset() {
	t.setName(fmt.Sprintf("Team %d", t.id))
	if t.id == 1 {
		t.setColor("Black")
	} else {
		t.setColor("White")
	}
	t.setScore(0)
	t.setLastScore(0)
	t.setTimeouts(3)
	t.setOfficialReviews(1)
	t.setOfficialReviewRetained(false)
	t.setLead(leadNo)
	t.setStarPass(false)
	t.setJammer("")
	t.setPivot("")
}

func (t *team) deleteSkater(data []string) error {
	if len(data) < 1 {
		return errSkaterNotFound
	}

	if _, ok := t.skaters[data[0]]; !ok {
		return errSkaterNotFound
	}
	delete(t.skaters, data[0])
	return state.StateDelete(t.base + ".Skater(" + data[0] + ")")
}

func (t *team) stateBase() string {
	return t.base
}

func (t *team) startTimeout(_ []string) error {
	state := stateTTO1
	if t.id == 2 {
		state = stateTTO2
	}
	return t.sb.timeout([]string{state})
}

func (t *team) startOfficialReview(_ []string) error {
	state := stateOR1
	if t.id == 2 {
		state = stateOR2
	}
	return t.sb.timeout([]string{state})
}

func (t *team) retainOfficialReview(_ []string) error {
	if t.officialReviews == 0 && !t.officialReviewRetained {
		t.setOfficialReviews(1)
		t.setOfficialReviewRetained(true)
	} else if t.officialReviews == 1 && t.officialReviewRetained {
		t.setOfficialReviews(0)
		t.setOfficialReviewRetained(false)
	}
	return nil
}

func (t *team) setName(v string) error {
	t.name = v
	return state.StateUpdateString(t.stateIDs["name"], v)
}

func (t *team) setColor(v string) error {
	t.color = v
	return state.StateUpdateString(t.stateIDs["color"], v)
}

func (t *team) setScore(v int64) error {
	if v < 0 {
		return nil
	}
	t.score = v
	if v < t.lastScore {
		t.setLastScore(v)
	}
	state.StateUpdateInt64(t.stateIDs["score"], v)
	state.StateUpdateInt64(t.stateIDs["jamScore"], t.score-t.lastScore)
	return nil
}

func (t *team) setLastScore(v int64) error {
	if v < 0 {
		return nil
	}
	if v > t.score {
		return nil
	}
	t.lastScore = v
	state.StateUpdateInt64(t.stateIDs["lastScore"], v)
	state.StateUpdateInt64(t.stateIDs["jamScore"], t.score-t.lastScore)
	return nil
}

func (t *team) setTimeouts(v int64) error {
	t.timeouts = v
	state.StateUpdateInt64(t.stateIDs["timeouts"], v)
	return nil
}

func (t *team) setOfficialReviews(v int64) error {
	t.officialReviews = v
	state.StateUpdateInt64(t.stateIDs["officialReviews"], v)
	return nil
}

func (t *team) setOfficialReviewRetained(v bool) error {
	t.officialReviewRetained = v
	return state.StateUpdateBool(t.stateIDs["officialReviewRetained"], v)
}

func (t *team) setLead(v string) error {
	t.lead = v
	return state.StateUpdateString(t.stateIDs["lead"], v)
}

func (t *team) setStarPass(v bool) error {
	t.starPass = v
	return state.StateUpdateBool(t.stateIDs["starPass"], v)
}

func (t *team) setJammer(v string) error {
	s, ok := t.skaters[v]
	if !ok {
		return errSkaterNotFound
	}
	return s.setPosition(positionJammer)
}

func (t *team) setJammerInBox(v bool) error {
	s, ok := t.skaters[t.jammer]
	if !ok {
		return errSkaterNotFound
	}
	return s.setInBox(v)
}

func (t *team) setPivot(v string) error {
	s, ok := t.skaters[v]
	if !ok {
		return errSkaterNotFound
	}
	return s.setPosition(positionPivot)
}

func (t *team) setPivotInBox(v bool) error {
	s, ok := t.skaters[t.pivot]
	if !ok {
		return errSkaterNotFound
	}
	return s.setInBox(v)
}

func (t *team) updatePositions() {
	t.jammer = ""
	t.pivot = ""
	state.StateDelete(t.base + ".Jammer")
	state.StateDelete(t.base + ".Pivot")
	t.sb.activeJam.clearTeamPositions(t)
	for _, s := range t.skaters {
		t.sb.activeJam.setTeamPosition(s)
		if s.position == positionJammer {
			t.jammer = s.id
			state.StateUpdateString(t.base+".Jammer.ID", s.id)
			state.StateUpdateString(t.base+".Jammer.Name", s.name)
			state.StateUpdateString(t.base+".Jammer.Number", s.number)
			state.StateUpdateBool(t.base+".Jammer.InBox", s.inBox())
		} else if s.position == positionPivot {
			t.pivot = s.id
			state.StateUpdateString(t.base+".Pivot.ID", s.id)
			state.StateUpdateString(t.base+".Pivot.Name", s.name)
			state.StateUpdateString(t.base+".Pivot.Number", s.number)
			state.StateUpdateBool(t.base+".Pivot.InBox", s.inBox())
		}
	}
}

func (t *team) useTimeout() bool {
	if t.timeouts > 0 {
		t.setTimeouts(t.timeouts - 1)
		return true
	}
	return false
}

func (t *team) useOfficialReview() bool {
	if t.officialReviews > 0 {
		t.setOfficialReviews(t.officialReviews - 1)
		return true
	}
	return false
}

func (t *team) incScore(_ []string) error {
	t.setScore(t.score + 1)
	return nil
}

func (t *team) decScore(_ []string) error {
	t.setScore(t.score - 1)
	return nil
}

func (t *team) incLastScore(_ []string) error {
	t.setLastScore(t.lastScore + 1)
	return nil
}

func (t *team) decLastScore(_ []string) error {
	t.setLastScore(t.lastScore - 1)
	return nil
}
