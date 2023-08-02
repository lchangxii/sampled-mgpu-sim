package utils

import (
    "flag"
)
var ArchFlag = flag.String("arch", "r9nano",
	"The GPU architecture types; we now support r9nano and MI100.")

