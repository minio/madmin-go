// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"math"
	"testing"
)

func TestParseSampleRate(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedValue  float64
		expectedErr    bool
		expectedErrMsg string
	}{
		// Valid fraction inputs
		{
			name:          "valid fraction 1/10",
			input:         "1/10",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "valid fraction 3/4",
			input:         "3/4",
			expectedValue: 0.75,
			expectedErr:   false,
		},
		{
			name:          "valid fraction with decimals 2.5/5",
			input:         "2.5/5",
			expectedValue: 0.5,
			expectedErr:   false,
		},

		// Invalid fraction inputs
		{
			name:           "invalid fraction too many slashes",
			input:          "1/2/3",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (1/2/3)",
		},
		{
			name:           "invalid fraction empty numerator",
			input:          "/2",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (/2)",
		},
		{
			name:           "invalid fraction empty denominator",
			input:          "1/",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (1/)",
		},
		{
			name:           "invalid fraction non-numeric numerator",
			input:          "abc/2",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (abc/2)",
		},
		{
			name:           "invalid fraction non-numeric denominator",
			input:          "2/abc",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (2/abc)",
		},
		{
			name:           "invalid fraction zero denominator",
			input:          "1/0",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (1/0)",
		},

		// Valid percentage inputs
		{
			name:          "valid percentage 10%",
			input:         "10%",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "valid percentage 100%",
			input:         "100%",
			expectedValue: 1.0,
			expectedErr:   false,
		},
		{
			name:          "valid percentage with decimal 25.5%",
			input:         "25.5%",
			expectedValue: 0.255,
			expectedErr:   false,
		},

		// Invalid percentage inputs
		{
			name:           "invalid percentage empty",
			input:          "%",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (%)",
		},
		{
			name:           "invalid percentage non-numeric",
			input:          "abc%",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (abc%)",
		},
		{
			name:           "invalid percentage negative",
			input:          "-10%",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (-10%)",
		},
		{
			name:           "invalid percentage multiple %",
			input:          "10%%",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (10%%)",
		},

		// Valid float inputs
		{
			name:          "valid float 0.1",
			input:         "0.1",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "valid float 1",
			input:         "1",
			expectedValue: 1.0,
			expectedErr:   false,
		},
		{
			name:          "valid float 123.456",
			input:         "123.456",
			expectedValue: 123.456,
			expectedErr:   false,
		},

		// Invalid float inputs
		{
			name:           "invalid float non-numeric",
			input:          "abc",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "strconv.ParseFloat: parsing \"abc\": invalid syntax",
		},
		{
			name:           "invalid float empty",
			input:          "",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "strconv.ParseFloat: parsing \"\": invalid syntax",
		},

		// Edge cases and potential bugs
		{
			name:          "fraction with negative numerator",
			input:         "-1/2",
			expectedValue: -0.5,
			expectedErr:   false, // Potential bug: code doesn't reject negative numerators
		},
		{
			name:          "percentage with trailing whitespace",
			input:         "10% ",
			expectedValue: 0.1, // Will fail: code doesn't handle trailing whitespace
			expectedErr:   false,
		},
		{
			name:          "float with leading whitespace",
			input:         " 0.1",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:           "ambiguous input with / and %",
			input:          "10%/100",
			expectedValue:  0, // Interpreted as fraction due to order of checks
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (10%/100)",
		},
		// Valid percentages with whitespace
		{
			name:          "percentage with space before percent",
			input:         "10 %",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "percentage with trailing space",
			input:         "10% ",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "percentage with leading space",
			input:         " 10%",
			expectedValue: 0.1,
			expectedErr:   false,
		},

		// Valid plain floats with whitespace
		{
			name:          "plain float with leading space",
			input:         " 0.1",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "plain float with trailing space",
			input:         "0.1 ",
			expectedValue: 0.1,
			expectedErr:   false,
		},
		{
			name:          "plain float with spaces around",
			input:         " 0.1 ",
			expectedValue: 0.1,
			expectedErr:   false,
		},

		// Invalid inputs with whitespace
		{
			name:           "invalid fraction with trailing space only",
			input:          "1 / ",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate (1 / )",
		},
		{
			name:           "invalid percentage with space and no number",
			input:          " %",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "invalid sample rate ( %)",
		},
		{
			name:           "empty string with spaces",
			input:          "   ",
			expectedValue:  0,
			expectedErr:    true,
			expectedErrMsg: "strconv.ParseFloat: parsing \"\": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotErr := ParseSampleRate(tt.input)

			// Check error
			if tt.expectedErr {
				if gotErr == nil {
					t.Errorf("ParseSampleRate(%q) expected error %q, got nil", tt.input, tt.expectedErrMsg)
				} else if gotErr.Error() != tt.expectedErrMsg {
					t.Errorf("ParseSampleRate(%q) expected error message %q, got %q", tt.input, tt.expectedErrMsg, gotErr.Error())
				}
			} else {
				if gotErr != nil {
					t.Errorf("ParseSampleRate(%q) unexpected error: %v", tt.input, gotErr)
				}
			}

			// Check value (only if no error expected)
			if !tt.expectedErr {
				if math.Abs(gotValue-tt.expectedValue) > 1e-9 { // Using epsilon for float comparison
					t.Errorf("ParseSampleRate(%q) expected value %v, got %v", tt.input, tt.expectedValue, gotValue)
				}
			}
		})
	}
}
