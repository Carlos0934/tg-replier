package commands

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		// --- backward compatibility (unquoted) ---
		{
			name:  "unquoted arguments",
			input: `/reply @alice hello`,
			want:  []string{"/reply", "@alice", "hello"},
		},
		{
			name:  "extra whitespace",
			input: `  /start  `,
			want:  []string{"/start"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},

		// --- double-quoted argument ---
		{
			name:  "double-quoted multi-word arg",
			input: `/group set "team alpha" @alice`,
			want:  []string{"/group", "set", "team alpha", "@alice"},
		},

		// --- single-quoted argument ---
		{
			name:  "single-quoted multi-word arg",
			input: `/msg 'hello world' @bob`,
			want:  []string{"/msg", "hello world", "@bob"},
		},

		// --- escaped quotes ---
		{
			name:  "escaped double quote inside double-quoted",
			input: `/note "it\"s fine"`,
			want:  []string{"/note", `it"s fine`},
		},
		{
			name:  "escaped single quote inside single-quoted",
			input: `/note 'it\'s fine'`,
			want:  []string{"/note", "it's fine"},
		},

		// --- mixed quoted and unquoted ---
		{
			name:  "unquoted before quoted",
			input: `/add leader "Jane Doe"`,
			want:  []string{"/add", "leader", "Jane Doe"},
		},
		{
			name:  "quoted before unquoted",
			input: `/send "hello there" now`,
			want:  []string{"/send", "hello there", "now"},
		},

		// --- malformed input ---
		{
			name:    "unmatched double quote",
			input:   `/group set "team alpha`,
			wantErr: "malformed command: unmatched quote",
		},
		{
			name:    "unmatched single quote",
			input:   `/msg 'hello world`,
			wantErr: "malformed command: unmatched quote",
		},

		// --- empty quoted string ---
		{
			name:  "empty double-quoted string",
			input: `/set name ""`,
			want:  []string{"/set", "name", ""},
		},
		{
			name:  "empty single-quoted string",
			input: `/set name ''`,
			want:  []string{"/set", "name", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				if got != nil {
					t.Fatalf("expected nil tokens on error, got %v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("token count: got %d (%v), want %d (%v)", len(got), got, len(tt.want), tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
