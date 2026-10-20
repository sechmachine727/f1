package protocol

import (
	"fmt"
	"sort"

	"f1tele/server/internal/contract"
)

// ChannelSpec binds a canonical channel to its official wire field.
type ChannelSpec struct {
	contract.Channel
	PacketID   uint8
	Field      string
	CarIndexed bool
	Corner     int // -1 none, else index into the UDP wheel order RL, RR, FL, FR
	Scale      float64
	Offset     float64
	Only2026   bool
	Restricted bool // subject to the participant m_yourTelemetry restriction
}

const psiToPa = 6894.757293168

var wheelNames = [4]string{"rl", "rr", "fl", "fr"}

var canonical = func() []ChannelSpec {
	var s []ChannelSpec
	add := func(name, unit, typ string, pid uint8, field string, car bool, corner int, scale float64, restricted bool, class string) {
		s = append(s, ChannelSpec{
			Channel:  contract.Channel{Name: name, Unit: unit, ValueType: typ, SourceFields: []string{fmt.Sprintf("packet%d.%s", pid, field)}, AvailabilityRule: availabilityFor(restricted), FrequencyClass: class},
			PacketID: pid, Field: field, CarIndexed: car, Corner: corner, Scale: scale, Restricted: restricted,
		})
	}
	add("car.speed_mps", "m/s", "number", 6, "m_speed", true, -1, 1.0/3.6, false, "high_rate")
	add("car.throttle_fraction", "fraction", "number", 6, "m_throttle", true, -1, 1, false, "high_rate")
	add("car.brake_fraction", "fraction", "number", 6, "m_brake", true, -1, 1, false, "high_rate")
	add("car.steer_fraction", "fraction", "number", 6, "m_steer", true, -1, 1, false, "high_rate")
	add("car.gear", "gear", "integer", 6, "m_gear", true, -1, 1, false, "high_rate")
	add("car.rpm", "rpm", "integer", 6, "m_engineRPM", true, -1, 1, false, "high_rate")
	add("car.drs", "flag", "boolean", 6, "m_drs", true, -1, 1, false, "high_rate")
	add("car.engine_temp_c", "degC", "number", 6, "m_engineTemperature", true, -1, 1, false, "status")
	for i, pos := range wheelNames {
		add("tyre."+pos+".surface_c", "degC", "number", 6, "m_tyresSurfaceTemperature", true, i, 1, true, "high_rate")
		add("tyre."+pos+".core_c", "degC", "number", 6, "m_tyresInnerTemperature", true, i, 1, true, "high_rate")
		add("tyre."+pos+".pressure_pa", "Pa", "number", 6, "m_tyresPressure", true, i, psiToPa, true, "high_rate")
		add("tyre."+pos+".wear_pct", "%", "number", 10, "m_tyresWear", true, i, 1, true, "status")
		add("tyre."+pos+".damage_pct", "%", "number", 10, "m_tyresDamage", true, i, 1, true, "status")
		add("tyre."+pos+".blisters_pct", "%", "number", 10, "m_tyreBlisters", true, i, 1, true, "status")
		add("brake."+pos+".temp_c", "degC", "number", 6, "m_brakesTemperature", true, i, 1, true, "high_rate")
		add("brake."+pos+".damage_pct", "%", "number", 10, "m_brakesDamage", true, i, 1, true, "status")
	}
	add("energy.store_j", "J", "number", 7, "m_ersStoreEnergy", true, -1, 1, true, "high_rate")
	add("energy.deploy_mode", "mode", "string", 7, "m_ersDeployMode", true, -1, 1, true, "high_rate")
	add("energy.harvest_limit_j_per_lap", "J", "number", 7, "m_ersHarvestLimitPerLap", true, -1, 1, true, "status")
	add("fuel.mass_kg", "kg", "number", 7, "m_fuelInTank", true, -1, 1, true, "status")
	add("fuel.capacity_kg", "kg", "number", 7, "m_fuelCapacity", true, -1, 1, true, "status")
	add("fuel.finish_margin_laps", "laps", "number", 7, "m_fuelRemainingLaps", true, -1, 1, true, "status")
	add("car.fia_flag", "flag", "integer", 7, "m_vehicleFIAFlags", true, -1, 1, false, "status")
	add("tyre.compound", "compound", "integer", 7, "m_actualTyreCompound", true, -1, 1, false, "status")
	add("tyre.age_laps", "laps", "integer", 7, "m_tyresAgeLaps", true, -1, 1, false, "status")
	add("timing.lap_ms", "ms", "integer", 2, "m_lastLapTimeInMS", true, -1, 1, false, "status")
	add("timing.current_lap_num", "lap", "integer", 2, "m_currentLapNum", true, -1, 1, false, "status")
	add("timing.current_lap_ms", "ms", "integer", 2, "m_currentLapTimeInMS", true, -1, 1, false, "high_rate")
	add("timing.lap_distance_m", "m", "number", 2, "m_lapDistance", true, -1, 1, false, "high_rate")
	add("timing.car_position", "position", "integer", 2, "m_carPosition", true, -1, 1, false, "status")
	add("timing.pit_status", "status", "integer", 2, "m_pitStatus", true, -1, 1, false, "status")
	add("timing.gap_ahead_s", "s", "number", 2, "derived:m_deltaToCarInFront", true, -1, 1, false, "status")
	add("timing.gap_leader_s", "s", "number", 2, "derived:m_deltaToRaceLeader", true, -1, 1, false, "status")
	add("timing.pit_stops", "stops", "integer", 2, "m_numPitStops", true, -1, 1, false, "status")
	add("timing.penalties_s", "s", "integer", 2, "m_penalties", true, -1, 1, false, "status")
	add("timing.warnings", "count", "integer", 2, "m_totalWarnings", true, -1, 1, false, "status")
	add("timing.speed_trap_mps", "m/s", "number", 2, "m_speedTrapFastestSpeed", true, -1, 1.0/3.6, false, "status")
	add("aero.active_mode", "mode", "string", 16, "m_activeAeroMode", true, -1, 1, false, "high_rate")
	add("aero.available", "flag", "boolean", 16, "m_activeAeroAvailable", true, -1, 1, false, "status")
	add("overtake.eligible", "flag", "boolean", 16, "m_overtakeAvailable", true, -1, 1, false, "status")
	add("boost.active", "flag", "boolean", 16, "m_overtakeActive", true, -1, 1, false, "high_rate")
	add("regulation.applies_2026", "flag", "boolean", 16, "m_2026Regulations", true, -1, 1, false, "session")
	add("session.weather", "code", "integer", 1, "m_weather", false, -1, 1, false, "session")
	add("session.track_temp_c", "degC", "number", 1, "m_trackTemperature", false, -1, 1, false, "session")
	add("session.air_temp_c", "degC", "number", 1, "m_airTemperature", false, -1, 1, false, "session")
	add("session.track_length_m", "m", "number", 1, "m_trackLength", false, -1, 1, false, "session")
	add("session.total_laps", "laps", "integer", 1, "m_totalLaps", false, -1, 1, false, "session")
	add("session.safety_car_status", "code", "integer", 1, "m_safetyCarStatus", false, -1, 1, false, "session")
	add("car.team_id", "team", "integer", 4, "m_teamId", true, -1, 1, false, "session")
	add("car.race_number", "number", "integer", 4, "m_raceNumber", true, -1, 1, false, "session")
	add("car.driver_name", "", "string", 4, "m_name", true, -1, 1, false, "session")
	add("car.is_ai", "flag", "boolean", 4, "m_aiControlled", true, -1, 1, false, "session")
	add("car.drs_fault_pct", "%", "number", 10, "m_drsFault", true, -1, 1, true, "status")
	add("car.ers_fault_pct", "%", "number", 10, "m_ersFault", true, -1, 1, true, "status")
	add("car.wing_front_left_damage_pct", "%", "number", 10, "m_frontLeftWingDamage", true, -1, 1, true, "status")
	add("car.wing_front_right_damage_pct", "%", "number", 10, "m_frontRightWingDamage", true, -1, 1, true, "status")
	add("car.wing_rear_damage_pct", "%", "number", 10, "m_rearWingDamage", true, -1, 1, true, "status")
	add("car.floor_damage_pct", "%", "number", 10, "m_floorDamage", true, -1, 1, true, "status")
	add("car.diffuser_damage_pct", "%", "number", 10, "m_diffuserDamage", true, -1, 1, true, "status")
	add("car.sidepod_damage_pct", "%", "number", 10, "m_sidepodDamage", true, -1, 1, true, "status")
	add("car.gearbox_damage_pct", "%", "number", 10, "m_gearBoxDamage", true, -1, 1, true, "status")
	add("car.engine_damage_pct", "%", "number", 10, "m_engineDamage", true, -1, 1, true, "status")
	return s
}()

func availabilityFor(restricted bool) string {
	if restricted {
		return "available for the player and unrestricted senders; restricted for participants with m_yourTelemetry=0"
	}
	return "available when received and fresh"
}

var canonicalByPacket = func() map[uint8][]ChannelSpec {
	m := map[uint8][]ChannelSpec{}
	for _, c := range canonical {
		m[c.PacketID] = append(m[c.PacketID], c)
	}
	return m
}()

// ByPacket returns the canonical channels carried by one packet ID.
func ByPacket(pid uint8) []ChannelSpec { return canonicalByPacket[pid] }

var canonicalByName = func() map[string]ChannelSpec {
	m := map[string]ChannelSpec{}
	for _, c := range canonical {
		m[c.Name] = c
	}
	return m
}()

// Canonical returns the canonical channel definitions in stable order.
func Canonical() []ChannelSpec { return canonical }

// Lookup returns a canonical channel definition.
func Lookup(name string) (ChannelSpec, bool) {
	c, ok := canonicalByName[name]
	return c, ok
}

// Catalog returns the canonical channels plus one raw mapping for every decoded
// official field not already exposed by a canonical channel.
func Catalog() []contract.Channel {
	out := make([]contract.Channel, 0, len(canonical)+200)
	mapped := map[string]bool{}
	for _, c := range canonical {
		out = append(out, c.Channel)
		mapped[c.Field] = true
	}
	for _, format := range []uint16{2025, 2026} {
		pids := make([]int, 0, len(compiled[format]))
		for pid := range compiled[format] {
			pids = append(pids, int(pid))
		}
		sort.Ints(pids)
		for _, pidInt := range pids {
			pid := uint8(pidInt)
			segs := layouts[format][pid]
			for _, seg := range segs {
				for _, f := range seg.fields {
					if mapped[f.name] || seg.name == "header" {
						continue
					}
					mapped[f.name] = true
					out = append(out, contract.Channel{
						Name:             fmt.Sprintf("raw.%d.%s", pid, f.name),
						Unit:             "",
						ValueType:        rawType(f.kind),
						SourceFields:     []string{fmt.Sprintf("packet%d.%s", pid, f.name)},
						AvailabilityRule: "raw official field; availability follows the packet and restriction rules",
						FrequencyClass:   "status",
					})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func rawType(k kind) string {
	if k == kStr {
		return "string"
	}
	return "number"
}

// DeployModeLabel maps the raw ERS deploy mode enum to its 2025 label.
func DeployModeLabel(v float64) string {
	switch int(v) {
	case 0:
		return "None"
	case 1:
		return "Medium"
	case 2:
		return "Hotlap"
	case 3:
		return "Overtake"
	}
	return "Unknown"
}

// DeployModeLabelForSeason names mode 3 by the session's regulation. The 2025
// structures call it overtake; the 2026 pack calls it boost. A session without
// a 2025 or 2026 regulation keeps the raw mode number rather than guessing.
func DeployModeLabelForSeason(v float64, season *int) string {
	if int(v) == 3 && (season == nil || (*season != 2025 && *season != 2026)) {
		return "Mode 3 (season unknown)"
	}
	if int(v) == 3 && *season == 2026 {
		return "Boost"
	}
	return DeployModeLabel(v)
}

// ActiveAeroLabel maps the 2026 active-aero mode enum to a label.
func ActiveAeroLabel(v float64) string {
	if int(v) == 1 {
		return "Straight"
	}
	if int(v) == 0 {
		return "Corner"
	}
	return "Unknown"
}

// CompoundLabel maps the actual-compound IDs to the published names. Visual
// compounds use a separate numbering and are not accepted here.
func CompoundLabel(v int) string {
	switch v {
	case 7:
		return "Inter"
	case 8:
		return "Wet"
	case 9:
		return "Dry (classic)"
	case 10:
		return "Wet (classic)"
	case 11:
		return "Super soft"
	case 12:
		return "Soft"
	case 13:
		return "Medium"
	case 14:
		return "Hard"
	case 15:
		return "Wet (F2)"
	case 16:
		return "C5"
	case 17:
		return "C4"
	case 18:
		return "C3"
	case 19:
		return "C2"
	case 20:
		return "C1"
	case 21:
		return "C0"
	case 22:
		return "C6"
	}
	return fmt.Sprintf("Compound %d", v)
}
