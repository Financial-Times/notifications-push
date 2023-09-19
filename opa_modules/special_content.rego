package specialContent

import future.keywords.if

default allow := true

allow = false if {
    input.EditorialDesk == "/FT/Professional/Central Banking"
}