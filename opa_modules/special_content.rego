package specialContent

import future.keywords.if
import future.keywords.in

default allow := true

allow = false if {
    input.EditorialDesk == "/FT/Professional/Central Banking"
}

# Disable notifications for listed publications
# Sustainable Views => 8e6c705e-1132-42a2-8db0-c295e29e8658
# FTA => 19d50190-8656-4e91-8d34-82e646ada9c9
block_notication_for_publication := ["8e6c705e-1132-42a2-8db0-c295e29e8658","19d50190-8656-4e91-8d34-82e646ada9c9"]
allow = false  if {
	input.Publication in block_notication_for_publication
}

