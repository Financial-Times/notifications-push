package publication

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type publicationBodyTest struct {
	Publication *Publications `json:"publication,omitempty"`
}

func TestPublication(t *testing.T) {
	tests := []struct {
		name             string
		publicationUuids string
		expectedValid    bool
		expectedAsOne    string
		expectedError    error
	}{
		{
			"Valid Sustainable Views publication",
			"[\"8e6c705e-1132-42a2-8db0-c295e29e8658\"]",
			true,
			"8e6c705e-1132-42a2-8db0-c295e29e8658",
			nil,
		},
		{
			"Valid FT Pink publication",
			"[\"88fdde6c-2aa4-4f78-af02-9f680097cfd6\"]",
			true,
			"88fdde6c-2aa4-4f78-af02-9f680097cfd6",
			nil,
		},
		{
			"Invalid publication",
			"[\"43ec9e35-763b-4568-8e8e-1a301b51facb\"]",
			false,
			"43ec9e35-763b-4568-8e8e-1a301b51facb",
			nil,
		},
		{
			"Valid Sustainable Views publication and FT Pink",
			"[\"8e6c705e-1132-42a2-8db0-c295e29e8658\", \"88fdde6c-2aa4-4f78-af02-9f680097cfd6\"]",
			true,
			"88fdde6c-2aa4-4f78-af02-9f680097cfd6",
			nil,
		},
		{
			"MoreThanOne error list without FT Pink",
			"[\"19002800-19fe-4486-8462-e5fb815b6246\", \"43ec9e35-763b-4568-8e8e-1a301b51facb\"]",
			false,
			"",
			ErrMoreThanOne,
		},
		{
			"Empty publication list",
			"[]",
			false,
			"",
			ErrEmptyPubList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody := []byte(fmt.Sprintf("{\"publication\": %s}", tt.publicationUuids))
			var pb publicationBodyTest
			err := json.Unmarshal(jsonBody, &pb)
			assert.NoError(t, err)
			_, v := Verify(*pb.Publication)
			assert.Equal(t, tt.expectedValid, v, "Publication must be valid.")
			pu, err := pb.Publication.OnlyOneOrPink()
			assert.Equal(t, tt.expectedAsOne, pu, "Expected AsOne must be equal")
			assert.ErrorIs(t, tt.expectedError, err, "OnlyOneAsString error")
		})
	}
}
