package encode

import "strings"

// QualityFor maps a quality preset to the encoder's value. VideoToolbox q:v
// runs 0-100 (higher = better), software encoders use CRF (lower = better).
func QualityFor(encoder, preset string) int {
	vt := strings.Contains(encoder, "videotoolbox")
	switch preset {
	case "quality":
		if vt {
			return 50
		}
		return 18
	case "speed":
		if vt {
			return 75
		}
		return 23
	default: // balanced
		if vt {
			return 65
		}
		return 20
	}
}
