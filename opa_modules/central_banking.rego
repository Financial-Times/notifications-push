package centralBanking

import future.keywords.if

default isCentralBanking := false

isCentralBanking if {
    input.resource["editorialDesk"] == "/FT/Professional/Central Banking"
}