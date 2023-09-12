package centralBanking

import future.keywords.if

default isCentralBanking := false

isCentralBanking if {
    input.EditorialDesk == "/FT/Professional/Central Banking"
}