package tests

import "testing"

type TestFrame struct {
	Name string
	Fn   func(t *testing.T)
}

type TestFrames []TestFrame

func (tt TestFrames) Run(t *testing.T) {
	for _, test := range tt {
		t.Run(test.Name, test.Fn)
	}
}
