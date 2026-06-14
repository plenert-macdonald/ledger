package ledger

import (
	"testing"
)

func Test_findMatchingAccount(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		trainingLedger   []*Transaction
		accountSubstring string
		want             string
		wantErr          bool
	}{
		{
			"simple test",
			[]*Transaction{
				&Transaction{
					AccountChanges: []Account{
						{Name: "Equity:Fake"},
						{Name: "Liability:Real"},
					},
				},
			},
			"Fake",
			"Equity:Fake",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &Importer{trainingLedger: tt.trainingLedger}
			got, gotErr := imp.findMatchingAccount(tt.accountSubstring)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("findMatchingAccount() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("findMatchingAccount() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if got != tt.want {
				t.Errorf("findMatchingAccount() = %v, want %v", got, tt.want)
			}
		})
	}
}
