package luhn

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		wantErr bool
	}{
		{
			name:    "1",
			number:  "2000000000008",
			wantErr: false,
		},
		{
			name:    "1",
			number:  "1000000000009",
			wantErr: false,
		},
		{
			name:    "1",
			number:  "3000000000007",
			wantErr: false,
		},
		{
			name:    "1",
			number:  "23423423",
			wantErr: true,
		},
		{
			name:    "1",
			number:  "7777",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.number); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
