package temper

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRefactorExactMatch(t *testing.T) {
	type in struct {
		V string
	}
	type out struct {
		V string
	}

	refactor := RefactorArgs[in, out]{
		Name: "test",
		New: func(args in) out {
			return out(args)
		},
		Old: func(args in) out {
			return out(args)
		},
	}

	actual := refactor.run(in{V: "test"})
	expected := out{V: "test"}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("results don't match, expected %v but got %v", expected, actual)
	}
}

func TestRefactor_results_exactMatch(t *testing.T) {
	type in struct {
		V string
	}
	type out struct {
		V string
	}

	refactor := RefactorArgs[in, out]{
		Name: "test",
		New: func(args in) out {
			return out(args)
		},
		Old: func(args in) out {
			return out(args)
		},
	}

	refactor.run(in{V: "test"})

	actual := refactor.results()
	expected := &addRefactorResultRequest{
		Key: "test",
		ResultParameters: []*refactorResultParameters{
			{
				ArgsType: "temper.in",
				Args: []*refactorParameter{
					{
						Name: "V",
						Type: "string",
						Value: "test",
					},
				},
				OldType: "temper.out",
				Old: []*refactorParameter{
					{
						Name: "V",
						Type: "string",
						Value: "test",
					},
				},
				NewType: "temper.out",
				New: []*refactorParameter{
					{
						Name: "V",
						Type: "string",
						Value: "test",
					},
				},
			},
		},
	}

	// We can't directly the entire struct using DeepEqual because the duration
	// will vary depending on where these tests are run, so we need to compare
	// each field.
	if expected.Key != actual.Key {
		t.Errorf("expected keys to match, got %s vs %s", expected.Key, actual.Key)
	}
	if !reflect.DeepEqual(expected.ResultParameters, actual.ResultParameters) {
		var allExpectedResultParameters string
		var allActualResultParameters string

		for _, rp := range expected.ResultParameters {
			allExpectedResultParameters += fmt.Sprintf("%#v ", rp)
		}
		for _, rp := range actual.ResultParameters {
			allActualResultParameters += fmt.Sprintf("%#v ", rp)
		}
		t.Fatalf("refactor result parameters don't match, expected:\n%s\n  but got:\n%s", allExpectedResultParameters, allActualResultParameters)
	}
}
