package token

import "time"

// ponytail: umur token tetap 24 jam; pindah ke env kalau perlu refresh flow
const ExpireDuration = 24 * time.Hour
