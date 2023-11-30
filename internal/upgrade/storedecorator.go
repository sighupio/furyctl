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
	}
)

type ReducersOperatorPhaseAsync[T Reducers] interface {
	ReducersOperatorPhase[T]
	Stop() error
}

func NewReducerOperatorPhaseDecorator[T Reducers](
	storer Storer,
	phase ReducersOperatorPhase[T],
) *ReducerOperatorPhaseDecorator[T] {
	return &ReducerOperatorPhaseDecorator[T]{
		storer: storer,
		phase:  phase,
	}
}

type ReducerOperatorPhaseDecorator[T Reducers] struct {
	storer Storer
	phase  ReducersOperatorPhase[T]
}

func (d *ReducerOperatorPhaseDecorator[T]) Exec(reducers T, startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(reducers, startFrom, upgradeState)

	if sErr := d.storer.Store(upgradeState); sErr != nil {
		err := fmt.Errorf("error storing upgrade state: %w", sErr)

		if fnErr != nil {
			err = fmt.Errorf("%w, %w", err, fnErr)
		}

		return err
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func NewReducerOperatorPhaseAsyncDecorator[T Reducers](
	storer Storer,
	phase ReducersOperatorPhaseAsync[T],
) *ReducerOperatorPhaseAsyncDecorator[T] {
	return &ReducerOperatorPhaseAsyncDecorator[T]{
		storer: storer,
		phase:  phase,
	}
}

type ReducerOperatorPhaseAsyncDecorator[T Reducers] struct {
	storer Storer
	phase  ReducersOperatorPhaseAsync[T]
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Exec(reducers T, startFrom string, upgradeState *State) error { //nolint: lll,revive // confusing-naming is a false positive
	fnErr := d.phase.Exec(reducers, startFrom, upgradeState)

	if sErr := d.storer.Store(upgradeState); sErr != nil {
		err := fmt.Errorf("error storing upgrade state: %w", sErr)

		if fnErr != nil {
			err = fmt.Errorf("%w, %w", err, fnErr)
		}

		return err
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Stop() error {
	if err := d.phase.Stop(); err != nil {
		return fmt.Errorf("error while stopping phase: %w", err)
	}

	return nil
}

func (d *ReducerOperatorPhaseAsyncDecorator[T]) Decorated() *cluster.OperationPhase {
	var p any = d.phase

	return p.(*cluster.OperationPhase) //nolint: revive,forcetypeassert // it should be safe
}

type OperatorPhase interface {
	Exec(startFrom string, upgradeState *State) error
}

type OperatorPhaseAsync interface {
	OperatorPhase
	Stop() error
}

func NewOperatorPhaseDecorator(
	storer Storer,
	phase OperatorPhase,
) *OperatorPhaseDecorator {
	return &OperatorPhaseDecorator{
		storer: storer,
		phase:  phase,
	}
}

type OperatorPhaseDecorator struct {
	storer Storer
	phase  OperatorPhase
}

func (d *OperatorPhaseDecorator) Exec(startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(startFrom, upgradeState)

	if sErr := d.storer.Store(upgradeState); sErr != nil {
		err := fmt.Errorf("error storing upgrade state: %w", sErr)

		if fnErr != nil {
			err = fmt.Errorf("%w, %w", err, fnErr)
		}

		return err
	}

	if fnErr != nil {
		return fmt.Errorf("error while executing phase: %w", fnErr)
	}

	return nil
}

type OperatorPhaseAsyncDecorator struct {
	storer Storer
	phase  OperatorPhaseAsync
}

func NewOperatorPhaseAsyncDecorator(
	storer Storer,
	phase OperatorPhaseAsync,
) *OperatorPhaseAsyncDecorator {
	return &OperatorPhaseAsyncDecorator{
		storer: storer,
		phase:  phase,
	}
}

func (d *OperatorPhaseAsyncDecorator) Exec(startFrom string, upgradeState *State) error {
	fnErr := d.phase.Exec(startFrom, upgradeState)

	if sErr := d.storer.Store(upgradeState); sErr != nil {
		err := fmt.Errorf("error storing upgrade state: %w", sErr)

		if fnErr != nil {
			err = fmt.Errorf("%w, %w", err, fnErr)
		}

		return err
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

func (d *OperatorPhaseAsyncDecorator) Decorated() *cluster.OperationPhase {
	var p any = d.phase

	return p.(*cluster.OperationPhase) //nolint: revive,forcetypeassert // it should be safe
}
