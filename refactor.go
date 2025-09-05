package temper

import (
	"fmt"
	"log"
	"time"
	"reflect"
)

// A refactorParameter is an arbitrary refactor value. It represents both
// its inputs and outputs.
type refactorParameter struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type refactorResultParameters struct {
	ArgsType string       `json:"args_type"`
	Args     []*refactorParameter `json:"args"`
	OldType  string       `json:"old_type"`
	Old      []*refactorParameter `json:"old"`
	NewType  string       `json:"new_type"`
	New      []*refactorParameter `json:"new"`
}

// addRefactorResultRequest is the data type for submitting Refactor results
// to the Temper API.
type addRefactorResultRequest struct {
	Key                string              `json:"key"` // Refactor key.
	OldAverageDuration time.Duration       `json:"old_average_duration"`
	NewAverageDuration time.Duration       `json:"new_average_duration"`
	ResultParameters   []*refactorResultParameters `json:"results"`
}

// A result is the result of a Refactor call.
type result[Args, Ret any] struct {
	args   Args
	old    Ret
	new    Ret
	olddur time.Duration
	newdur time.Duration
}

type RefactorArgs[Args, Ret any] struct {
	Name string

	Old func(args Args) Ret
	New func(args Args) Ret

	OldErr func(args Args) (Ret, error)
	NewErr func(args Args) (Ret, error)

	result *result[Args, Ret]
}

// TODO I need to copy below with the error returning variation, so something
// like `RunErr`.

// Run executes both the old and new functions defined in the refactor, and
// returns the results of the `Old` function.
func (r *RefactorArgs[Args, Ret]) run(args Args) Ret {
	start := time.Now()

	// TODO
	//
	// Run the `Old` within the same thread as this `Run` method was called,
	// but run `New` in its own goroutine.
	//
	// This will probably end up sucking because it will discard side
	// effects... but on the other hand, side effects are bad considering
	// that **both** methods will be called, so the side effects would both
	// race and compete.
	//
	// It might be better to restrict what's possible to a larger degree and
	// require two type parameters, both with the comparable constraint, where
	// one is the function argument and the other is the result type.

	// Initialize the result struct.
	r.result = &result[Args, Ret]{
		args: args,
	}

	// Run the `New` func in its own goroutine.
	ch := make(chan Ret)
	go func() {
		ch <- r.New(args)
		r.result.newdur = time.Since(start)
	}()

	r.result.old = r.Old(args)
	r.result.olddur = time.Since(start)

	// Block until we receive a result from the `New` goroutine.
	r.result.new = <-ch

	// Return the old result to preserve the previous behaviour that the
	// caller is expecting/using this for in the first place.
	return r.result.old
}

// results returns an API client friendly representation of the type T
func (r *RefactorArgs[Args, Ret]) results() *addRefactorResultRequest {
	argsType, args := extractParam(r.result.args)
	oldType, oldRet := extractParam(r.result.old)
	newType, newRet := extractParam(r.result.new)

	return &addRefactorResultRequest{
		Key:                r.Name,
		OldAverageDuration: r.result.olddur,
		NewAverageDuration: r.result.newdur,
		ResultParameters: []*refactorResultParameters{
			{
				ArgsType: argsType,
				Args:     args,
				OldType:  oldType,
				Old:      oldRet,
				NewType:  newType,
				New:      newRet,
			},
		},
	}
}

func extractParam(i any) (string, []*refactorParameter) {
	rv := reflect.Indirect(reflect.ValueOf(i))
	rt := rv.Type()

	if rv.Type().Kind() != reflect.Struct {
		log.Printf("[temper] type %s is not a struct\n", rv.Type().Kind())
		return "", nil
	}

	params := make([]*refactorParameter, 0)

	for n := range rt.NumField() {
		f := rt.Field(n)
		v := rv.FieldByName(f.Name).Interface()

		// TODO Need special case for timestamp potentially.
		params = append(params, &refactorParameter{
			Name:  f.Name,
			Type:  f.Type.String(),
			Value: fmt.Sprintf("%v", v),
		})
	}

	return fmt.Sprintf("%T", i), params
}
