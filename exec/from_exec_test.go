package exec

import (
	"testing"

	"gitlab.eng.vmware.com/vivienv/flare/script"
)

func TestExecFROM(t *testing.T) {
	tests := []execTest{
		{
			name: "Exec FROM with single arg",
			source: func() string {
				return "FROM local"
			},
			exec: func(s *script.Script) error {
				e := New(s)
				if err := e.Execute(); err != nil {
					return err
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runExecutorTest(t, test)
		})
	}
}
