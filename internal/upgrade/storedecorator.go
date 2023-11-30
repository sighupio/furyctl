package upgrade

import "fmt"

type Reducers = any

type ReducersOperatorPhase[T Reducers] interface {
	Exec(reducers T, startFrom string, upgradeState *State) error
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

	return fnErr
}

type OperatorPhase interface {
	Exec(startFrom string, upgradeState *State) error
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

	return fnErr
}
