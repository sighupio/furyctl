// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgrade

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/cluster"
)

type (
	Reducers                          = any
	ReducersOperatorPhase[T Reducers] interface {
		Exec(reducers T, startFrom string, upgradeState *State) error
		SetUpgrade(upgradeEnabled bool)
		Self() *cluster.OperationPhase
	}
)

type ReducersOperatorPhaseAsync[T Reducers] interface {
	ReducersOperatorPhase[T]
	Stop() error
}

func NewReducerOperatorPhaseDecorator[T Reducers](
	storer Storer,
	phase ReducersOperatorPhase[T],
	dryRun bool,
	upgr *Upgrade,
) *ReducerOperatorPhaseDecorator[T] {
	return &ReducerOperatorPhaseDecorator[T]{
		storer: storer,
		phase:  phase,
		dryRun: dryRun,
		upgr:   upgr,
	}
}

type ReducerOperatorPhaseDecorator[T Reducers] struct {
	storer Storer
	phase  ReducersOperatorPhase[T]
	dryRun bool
	upgr   *Upgrade
}

func (d *ReducerOperatorPhaseDecorator[T]) Exec(reducers T, startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(reducers, startFrom, upgradeState)

	if !d.dryRun && d.upgr.Enabled {
		if sErr := d.storer.Store(upgradeState); sErr != nil {
			err := fmt.Errorf("error storing upgrade state: %w", sErr)

			if fnErr != nil {
				err = fmt.Errorf("%w, %w", err, fnErr)
			}

			return err
		}
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func (d *ReducerOperatorPhaseDecorator[T]) SetUpgrade(upgradeEnabled bool) {
	d.upgr.Enabled = upgradeEnabled
}

func (d *ReducerOperatorPhaseDecorator[T]) Self() *cluster.OperationPhase {
	return d.phase.Self()
}

func NewReducerOperatorPhaseAsyncDecorator[T Reducers](
	storer Storer,
	phase ReducersOperatorPhaseAsync[T],
	dryRun bool,
	upgr *Upgrade,
) *ReducerOperatorPhaseAsyncDecorator[T] {
	return &ReducerOperatorPhaseAsyncDecorator[T]{
		storer: storer,
		phase:  phase,
		dryRun: dryRun,
		upgr:   upgr,
	}
}

type ReducerOperatorPhaseAsyncDecorator[T Reducers] struct {
	storer Storer
	phase  ReducersOperatorPhaseAsync[T]
	dryRun bool
	upgr   *Upgrade
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Exec(reducers T, startFrom string, upgradeState *State) error { //nolint: lll // confusing-naming is a false positive
	fnErr := d.phase.Exec(reducers, startFrom, upgradeState)

	if !d.dryRun && d.upgr.Enabled {
		if sErr := d.storer.Store(upgradeState); sErr != nil {
			err := fmt.Errorf("error storing upgrade state: %w", sErr)

			if fnErr != nil {
				err = fmt.Errorf("%w, %w", err, fnErr)
			}

			return err
		}
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) SetUpgrade(upgradeEnabled bool) { //nolint: lll // confusing-naming is a false positive
	d.upgr.Enabled = upgradeEnabled
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Stop() error {
	if err := d.phase.Stop(); err != nil {
		return fmt.Errorf("error while stopping phase: %w", err)
	}

	return nil
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Self() *cluster.OperationPhase { //nolint: lll // confusing-naming is a false positive
	return d.phase.Self()
}

type OperatorPhase interface {
	Exec(startFrom string, upgradeState *State) error
	Self() *cluster.OperationPhase
	SetUpgrade(upgradeEnabled bool)
}

type OperatorPhaseAsync interface {
	OperatorPhase
	Stop() error
}

func NewOperatorPhaseDecorator(
	storer Storer,
	phase OperatorPhase,
	dryRun bool,
	upgr *Upgrade,
) *OperatorPhaseDecorator {
	return &OperatorPhaseDecorator{
		storer: storer,
		phase:  phase,
		dryRun: dryRun,
		upgr:   upgr,
	}
}

type OperatorPhaseDecorator struct {
	storer Storer
	phase  OperatorPhase
	dryRun bool
	upgr   *Upgrade
}

func (d *OperatorPhaseDecorator) Exec(startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(startFrom, upgradeState)

	if !d.dryRun && d.upgr.Enabled {
		if sErr := d.storer.Store(upgradeState); sErr != nil {
			err := fmt.Errorf("error storing upgrade state: %w", sErr)

			if fnErr != nil {
				err = fmt.Errorf("%w, %w", err, fnErr)
			}

			return err
		}
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func (d *OperatorPhaseDecorator) SetUpgrade(upgradeEnabled bool) {
	d.upgr.Enabled = upgradeEnabled
}

func (d *OperatorPhaseDecorator) Self() *cluster.OperationPhase {
	return d.phase.Self()
}

type OperatorPhaseAsyncDecorator struct {
	storer Storer
	phase  OperatorPhaseAsync
	dryRun bool
	upgr   *Upgrade
}

func NewOperatorPhaseAsyncDecorator(
	storer Storer,
	phase OperatorPhaseAsync,
	dryRun bool,
	upgr *Upgrade,
) *OperatorPhaseAsyncDecorator {
	return &OperatorPhaseAsyncDecorator{
		storer: storer,
		phase:  phase,
		dryRun: dryRun,
		upgr:   upgr,
	}
}

func (d *OperatorPhaseAsyncDecorator) Exec(startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(startFrom, upgradeState)

	if !d.dryRun && d.upgr.Enabled {
		if sErr := d.storer.Store(upgradeState); sErr != nil {
			err := fmt.Errorf("error storing upgrade state: %w", sErr)

			if fnErr != nil {
				err = fmt.Errorf("%w, %w", err, fnErr)
			}

			return err
		}
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func (d *OperatorPhaseAsyncDecorator) Stop() error {
	if err := d.phase.Stop(); err != nil {
		return fmt.Errorf("error while stopping phase: %w", err)
	}

	return nil
}

func (d *OperatorPhaseAsyncDecorator) SetUpgrade(upgradeEnabled bool) {
	d.upgr.Enabled = upgradeEnabled
}

func (d *OperatorPhaseAsyncDecorator) Self() *cluster.OperationPhase {
	return d.phase.Self()
}
