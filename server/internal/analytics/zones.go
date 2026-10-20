package analytics

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed corner_zones.json
var zoneBytes []byte

type zone struct {
	Number int     `json:"number"`
	Norm   float64 `json:"norm"`
}

var (
	zoneOnce sync.Once
	zones    map[int][]zone
)

// zonesFor returns provisional corner positions for a track. They come from the
// bundled circuit outlines, not from a calibrated corner profile.
func zonesFor(trackID int) []zone {
	zoneOnce.Do(func() {
		var raw map[string]struct {
			Corners []zone `json:"corners"`
		}
		zones = map[int][]zone{}
		if json.Unmarshal(zoneBytes, &raw) != nil {
			return
		}
		for k, v := range raw {
			id := 0
			for _, ch := range k {
				if ch < '0' || ch > '9' {
					id = -1
					break
				}
				id = id*10 + int(ch-'0')
			}
			if id >= 0 {
				zones[id] = v.Corners
			}
		}
	})
	return zones[trackID]
}
