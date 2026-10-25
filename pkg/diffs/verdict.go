// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diffs

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/samber/lo"

	"github.com/sighupio/furyctl/internal/cluster"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// Verdict is what the distribution rules decide for one configuration change.
type Verdict string

const (
	VerdictNone              Verdict = "none"
	VerdictMigration         Verdict = "migration"
	VerdictNeedsConfirmation Verdict = "needs confirmation"
	VerdictBlocked           Verdict = "blocked"

	// The two ways the unsafe reducer check of a phase sees the changes, as reported
	// or expanded into leaves (see reducerCheck).
	checkReported reducerCheck = 0
	checkLeaves   reducerCheck = 1
)

// KindProfile holds the phases whose rules the apply of a cluster kind reads, and
// where it expands a whole-object change into its leaf fields before a rule check.
type KindProfile struct {
	// CheckPhases are the phases of the immutable and unsupported checks.
	CheckPhases []string
	// UnsafeReducerPhases are the phases of the unsafe reducer check, in the order
	// the apply of the kind runs them.
	UnsafeReducerPhases []string
	// ExpandedReducerPhases are the phases whose unsafe reducer check runs on the
	// expanded changes.
	ExpandedReducerPhases []string

	// The zero value runs every rule check. Apply runs them one at a time, in
	// different functions, and a check must not run where apply does not run it:
	// the immutable check logs the fromNodes conditions it cannot evaluate.
	skipImmutable   bool
	skipUnsupported bool
}

// OnlyImmutableCheck gives the profile that runs the immutable check alone.
func (p KindProfile) OnlyImmutableCheck() KindProfile {
	p.skipUnsupported = true
	p.UnsafeReducerPhases = nil

	return p
}

// OnlyUnsupportedCheck gives the profile that runs the unsupported check alone.
func (p KindProfile) OnlyUnsupportedCheck() KindProfile {
	p.skipImmutable = true
	p.UnsafeReducerPhases = nil

	return p
}

// OnlyUnsafeReducerCheck gives the profile that runs the unsafe reducer check alone.
func (p KindProfile) OnlyUnsafeReducerCheck() KindProfile {
	p.CheckPhases = nil

	return p
}

// ChangeVerdict is the verdict of one leaf change, with the facts apply decides on.
type ChangeVerdict struct {
	// Change is the leaf change.
	Change r3diff.Change
	// Reported is the change of the changelog that Change comes from. It differs
	// from Change only when a whole object was added or removed.
	Reported r3diff.Change
	Verdict  Verdict
	// Text is the reason of the unsupported transition or the description of the
	// rule behind the verdict. It never holds configuration values.
	Text string
	// Immutable is true when the immutable check of apply fails on this change.
	Immutable bool
	// Unsupported is true when the unsupported check of apply fails on this change.
	Unsupported bool
	// UnsafeReducers holds, per phase, the rules with unsafe reducers this change matches.
	UnsafeReducers map[string][]rules.Rule
}

// Path gives the dotted path of the leaf change, as the rules write it.
func (v ChangeVerdict) Path() string {
	return "." + strings.Join(v.Change.Path, ".")
}

// RuleDecisions holds the verdicts of a changelog and what each rule check of apply
// returns for it, so that apply takes its decisions from here.
type RuleDecisions struct {
	// Verdicts has one verdict per leaf change, in the order of the changelog.
	Verdicts []ChangeVerdict
	// ImmutableViolations are the errors of the immutable check: one per change as
	// reported that a rule declares immutable, phase after phase in the order of the
	// profile, as apply joins them.
	ImmutableViolations []error
	// UnsupportedViolations are the errors of the unsupported check: one per leaf
	// change and rule with a matching unsupported transition, phase after phase in
	// the order of the profile, as apply joins them.
	UnsupportedViolations []error
	// UnsafeReducers holds, per phase, the rules with unsafe reducers, once per change
	// each rule matches, in the order of the rules.
	UnsafeReducers map[string][]rules.Rule
}

// ProfileForKind gives the profile of a cluster kind (the kind field of furyctl.yaml).
// The values are the rule checks each apply runs: the two rule check functions of
// <kind>/create/preflight.go and the unsafe reducer lists of <kind>/creator.go. They
// are not the supported phases of the kind: Immutable reads no kubernetes reducer
// and EKSCluster reads the distribution ones only.
func ProfileForKind(kind string) (KindProfile, bool) {
	switch kind {
	case "OnPremises":
		return KindProfile{
			CheckPhases:           []string{cluster.OperationPhaseKubernetes, cluster.OperationPhaseDistribution},
			UnsafeReducerPhases:   []string{cluster.OperationPhaseDistribution, cluster.OperationPhaseKubernetes},
			ExpandedReducerPhases: []string{cluster.OperationPhaseKubernetes},
		}, true

	case "Immutable":
		return KindProfile{
			CheckPhases: []string{
				cluster.OperationPhaseInfrastructure,
				cluster.OperationPhaseKubernetes,
				cluster.OperationPhaseDistribution,
			},
			UnsafeReducerPhases:   []string{cluster.OperationPhaseInfrastructure, cluster.OperationPhaseDistribution},
			ExpandedReducerPhases: nil,
		}, true

	case "EKSCluster":
		return KindProfile{
			CheckPhases: []string{
				cluster.OperationPhaseInfrastructure,
				cluster.OperationPhaseKubernetes,
				cluster.OperationPhaseDistribution,
			},
			UnsafeReducerPhases:   []string{cluster.OperationPhaseDistribution},
			ExpandedReducerPhases: nil,
		}, true

	case "KFDDistribution":
		return KindProfile{
			CheckPhases:           []string{cluster.OperationPhaseDistribution},
			UnsafeReducerPhases:   []string{cluster.OperationPhaseDistribution},
			ExpandedReducerPhases: nil,
		}, true

	default:
		return KindProfile{}, false
	}
}

// ErrNoKindProfile is the error of a kind that has no rule profile.
var ErrNoKindProfile = errors.New("no rule profile for the kind")

// RequireProfileForKind gives the profile of a cluster kind, or ErrNoKindProfile when
// the kind has none. Apply reads its profile with it: the zero profile runs no rule
// check, so a kind without a profile must stop the apply instead of skipping the checks.
func RequireProfileForKind(kind string) (KindProfile, error) {
	profile, ok := ProfileForKind(kind)
	if !ok {
		return KindProfile{}, fmt.Errorf("%w: %q", ErrNoKindProfile, kind)
	}

	return profile, nil
}

// candidate is one leaf change under evaluation: its verdict so far and the texts of
// the rules it matched, one list per outcome. The verdict picks one text once every
// check has run, so a rule met later cannot overwrite the text of a stronger outcome.
type candidate struct {
	ChangeVerdict

	unsupportedReason string
	blocked           []string
	unsafe            []string
	safe              []string
}

// decided gives the verdict of the candidate: the first that applies of blocked,
// needs confirmation and migration, with the text of the first rule behind it. A
// candidate no check marked keeps the verdict none and an empty text.
func (c *candidate) decided() ChangeVerdict {
	switch {
	case c.Immutable || c.Unsupported:
		return c.with(VerdictBlocked, c.blockedText())

	case len(c.unsafe) > 0:
		return c.with(VerdictNeedsConfirmation, c.unsafe[0])

	case len(c.safe) > 0:
		return c.with(VerdictMigration, c.safe[0])

	default:
		return c.ChangeVerdict
	}
}

// with gives the verdict of the candidate with its outcome and text filled in.
func (c *candidate) with(verdict Verdict, text string) ChangeVerdict {
	decided := c.ChangeVerdict
	decided.Verdict, decided.Text = verdict, text

	return decided
}

// blockedText is the reason of the unsupported transition when there is one, else
// the description of the first rule that blocks the change.
func (c *candidate) blockedText() string {
	if c.unsupportedReason != "" {
		return c.unsupportedReason
	}

	return c.blocked[0]
}

// reducerCheck is how the unsafe reducer check of a phase sees the changes: as
// reported, or expanded into leaves, as the apply of the kind runs it.
type reducerCheck int

// reducerCheck gives how the unsafe reducer check of a phase sees the changes.
func (p KindProfile) reducerCheck(phase string) reducerCheck {
	if slices.Contains(p.ExpandedReducerPhases, phase) {
		return checkLeaves
	}

	return checkReported
}

// changelog gives the changelog the check runs on.
func (k reducerCheck) changelog(changelog r3diff.Changelog) r3diff.Changelog {
	if k == checkLeaves {
		return ExpandMapChanges(changelog)
	}

	return changelog
}

// change gives the change of a candidate the check matches a rule against.
func (k reducerCheck) change(c *candidate) r3diff.Change {
	if k == checkLeaves {
		return c.Change
	}

	return c.Reported
}

// ruleRun evaluates the rule checks of one kind over the candidates of a changelog.
type ruleRun struct {
	changelog  r3diff.Changelog
	extractor  rules.Extractor
	candidates []candidate
}

// Verdicts gives one verdict per leaf change of the changelog, in the order of the
// changelog, with the leaves of a whole-object change sorted by path. See DecideRules.
func Verdicts(changelog r3diff.Changelog, extractor rules.Extractor, profile KindProfile) []ChangeVerdict {
	return DecideRules(changelog, extractor, profile).Verdicts
}

// DecideRules runs the rule checks of the profile on the changelog. The changelog is
// the one apply checks (not expanded) and the extractor holds the configuration apply
// gives it: the current one in the preflight.
//
// Rule matching and the safe and fromNodes conditions are the ones of the extractor
// and of the checker assertions. Conditions that look at other changes always get
// the whole changelog; only the attribution of a rule to a change is done one
// change at a time.
func DecideRules(changelog r3diff.Changelog, extractor rules.Extractor, profile KindProfile) RuleDecisions {
	run := ruleRun{changelog: changelog, extractor: extractor, candidates: undecidedCandidates(changelog)}
	decisions := RuleDecisions{
		Verdicts:              nil,
		ImmutableViolations:   nil,
		UnsupportedViolations: nil,
		UnsafeReducers:        map[string][]rules.Rule{},
	}

	for _, phase := range profile.CheckPhases {
		if !profile.skipImmutable {
			immutableRules := extractor.FilterSafeImmutableRules(extractor.GetImmutableRules(phase), changelog)

			decisions.ImmutableViolations = append(decisions.ImmutableViolations, run.markImmutable(immutableRules)...)
		}

		if !profile.skipUnsupported {
			decisions.UnsupportedViolations = append(
				decisions.UnsupportedViolations,
				run.markUnsupported(extractor.GetUnsupportedRules(phase))...,
			)
		}
	}

	for _, phase := range profile.UnsafeReducerPhases {
		decisions.UnsafeReducers[phase] = run.markReducers(phase, profile.reducerCheck(phase))
	}

	decisions.Verdicts = lo.Map(run.candidates, func(c candidate, _ int) ChangeVerdict {
		return c.decided()
	})

	return decisions
}

// Undecided gives the leaf changes of the changelog with no verdict, as Verdicts lists
// them, for when the rules of the kind are not available.
func Undecided(changelog r3diff.Changelog) []ChangeVerdict {
	return lo.Map(undecidedCandidates(changelog), func(c candidate, _ int) ChangeVerdict {
		return c.ChangeVerdict
	})
}

// undecidedCandidates gives one candidate per leaf change of the changelog, in the
// order of the changelog, with no rule check run yet.
func undecidedCandidates(changelog r3diff.Changelog) []candidate {
	candidates := make([]candidate, 0, len(changelog))

	for _, reported := range changelog {
		leaves := ExpandMapChanges(r3diff.Changelog{reported})

		// The expansion walks a map, so its leaves come in no fixed order; sorting
		// them by path gives two runs the same list.
		slices.SortFunc(leaves, func(a, b r3diff.Change) int {
			return strings.Compare(strings.Join(a.Path, "."), strings.Join(b.Path, "."))
		})

		for _, leaf := range leaves {
			candidates = append(candidates, candidate{
				ChangeVerdict: ChangeVerdict{
					Change:         leaf,
					Reported:       reported,
					Verdict:        VerdictNone,
					Text:           "",
					Immutable:      false,
					Unsupported:    false,
					UnsafeReducers: map[string][]rules.Rule{},
				},
				unsupportedReason: "",
				blocked:           nil,
				unsafe:            nil,
				safe:              nil,
			})
		}
	}

	return candidates
}

// markImmutable runs the immutable check of apply, which does not expand: a rule is
// matched against the change as reported, and its result goes to every leaf of it.
// It gives the errors of the check as apply gets them: one per change as reported.
func (r *ruleRun) markImmutable(immutableRules []rules.Rule) []error {
	checker := new(BaseChecker)

	for _, rule := range immutableRules {
		for i := range r.candidates {
			c := &r.candidates[i]

			if len(checker.AssertImmutableViolations(r3diff.Changelog{c.Reported}, []string{rule.Path})) == 0 {
				continue
			}

			c.Immutable = true
			c.blocked = append(c.blocked, lo.FromPtr(rule.Description))
		}
	}

	// The errors come from the one call apply makes: the changelog as reported
	// against every immutable path of the phase.
	return checker.AssertImmutableViolations(r.changelog, rules.Paths(immutableRules))
}

// markUnsupported runs the unsupported check of apply, which expands: a rule is
// matched against the leaf change. It gives the errors of the check as apply gets
// them: one per leaf change and rule, leaf by leaf.
func (r *ruleRun) markUnsupported(unsupportedRules []rules.Rule) []error {
	var violations []error

	checker := new(BaseChecker)

	for i := range r.candidates {
		c := &r.candidates[i]

		for _, rule := range unsupportedRules {
			errs := checker.AssertReducerUnsupportedViolations(r3diff.Changelog{c.Change}, []rules.Rule{rule})
			if len(errs) == 0 {
				continue
			}

			violations = append(violations, errs...)

			c.Unsupported = true
			c.blocked = append(c.blocked, lo.FromPtr(rule.Description))

			// The reason shown is the one of the first unsupported rule the change matches.
			if c.unsupportedReason == "" {
				c.unsupportedReason, _ = isDiffUnsupported(c.Change, *rule.Unsupported)
			}
		}
	}

	return violations
}

// markReducers runs the unsafe reducer check of apply for one phase, on the changelog
// apply gives to that check: expanded into leaf changes or as reported. It gives the
// rules with unsafe reducers as apply gets them: once per change each rule matches.
func (r *ruleRun) markReducers(phase string, check reducerCheck) []rules.Rule {
	var unsafeRules []rules.Rule

	checked := check.changelog(r.changelog)

	for _, rule := range r.extractor.GetReducers(phase) {
		matching := r.matching(rule, check)
		if len(matching) == 0 {
			continue
		}

		// ReducerRulesByDiffs writes the values of the change it matches into the
		// reducers of the rule, which every copy of the rule shares, and the safe
		// conditions read them. This call therefore comes after the ones above, with
		// the changes apply gives it, so that the rule is judged, and its reducers
		// are left, as apply does.
		unsafe := r.extractor.UnsafeReducerRulesByDiffs([]rules.Rule{rule}, checked)
		unsafeRules = append(unsafeRules, unsafe...)

		for _, c := range matching {
			if len(unsafe) == 0 {
				c.safe = append(c.safe, lo.FromPtr(rule.Description))

				continue
			}

			c.UnsafeReducers[phase] = append(c.UnsafeReducers[phase], rule)
			c.unsafe = append(c.unsafe, lo.FromPtr(rule.Description))
		}
	}

	return unsafeRules
}

// matching gives the candidates whose change, as the check sees it, the rule matches.
func (r *ruleRun) matching(rule rules.Rule, check reducerCheck) []*candidate {
	var matching []*candidate

	for i := range r.candidates {
		c := &r.candidates[i]

		if len(r.extractor.ReducerRulesByDiffs([]rules.Rule{rule}, r3diff.Changelog{check.change(c)})) > 0 {
			matching = append(matching, c)
		}
	}

	return matching
}
