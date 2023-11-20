// Package publication repesents publication field in an Article or a Notification message
package publication

import (
	"strings"

	"github.com/google/uuid"
)

const PinkFt = "88fdde6c-2aa4-4f78-af02-9f680097cfd6"
const SustainableViews = "8e6c705e-1132-42a2-8db0-c295e29e8658"

// Hardcoded supported pulications as map
var publications = map[string]string{
	PinkFt:           "FT Pink",
	SustainableViews: "Sustainable Views",
}

type Publications struct {
	UUIDS []uuid.UUID
}

// UnmarshslJSON
func (p *Publications) UnmarshalJSON(b []byte) error {
	uuids := strings.Split(string(b), "\"")
	p.UUIDS = make([]uuid.UUID, 0)
	for _, u := range uuids {
		pu, err := uuid.Parse(u)
		if err == nil {
			p.UUIDS = append(p.UUIDS, pu)
		}
	}
	return nil
}

func (p Publications) String() string {
	s := ""
	for i, pu := range p.UUIDS {
		n, v := publications[pu.String()]
		if v {
			s += n
			if len(p.UUIDS) != i-1 {
				s += ", "
			}
		}
	}
	return s
}

// Check if provided publication UUID is supported
// NOTE: The plan is for the future to use more sophisticated mechanism fetching the publications
func Verify(ps Publications) (string, bool) {
	v := false
	for _, p := range ps.UUIDS {
		_, v = publications[p.String()]
		if !v {
			return p.String(), v
		}
	}
	return "", v
}

// Return string version of the uuids of the publication
// For now we expect only one publication in the list
// If there are many publications in the list, returns only FT Pink uuid if available
func (p Publications) OnlyOneOrPink() (string, error) {
	for _, v := range p.UUIDS {
		if v.String() == PinkFt {
			return PinkFt, nil
		}
	}
	if len(p.UUIDS) == 0 {
		return "", ErrEmptyPubList
	} else if len(p.UUIDS) == 1 {
		return p.UUIDS[0].String(), nil
	} else {
		return "", ErrMoreThanOne
	}
}
