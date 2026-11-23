package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/geoenv"
)

func replayEvidence(name string, args []string) error {
	command := flag.NewFlagSet(name, flag.ContinueOnError)
	path := command.String("input", "", "issued offline civilian exercise file")
	digest := command.String("sha256", "", "trusted SHA-256 of the exercise file")
	var manifest string
	if name == "replay-access" || name == "replay-regional" {
		command.StringVar(&manifest, "manifest-sha256", "", "trusted SHA-256 of the referenced environment manifest")
	}
	var knownAt, port, baselineFrom, studyFrom, studyTo, minimum, consecutive string
	if name == "replay-port" {
		command.StringVar(&port, "port", "", "PortWatch identifier: port24 or port2074")
		command.StringVar(&baselineFrom, "baseline-from", "", "baseline start (YYYY-MM-DD)")
		command.StringVar(&studyFrom, "from", "", "inclusive study start (YYYY-MM-DD)")
		command.StringVar(&studyTo, "to-exclusive", "", "exclusive study end (YYYY-MM-DD)")
		command.StringVar(&minimum, "minimum-weekdays", "4", "minimum matching weekdays")
		command.StringVar(&consecutive, "consecutive-days", "3", "consecutive in-range days to confirm return")
	} else {
		command.StringVar(&knownAt, "known-at", "", "cutoff as an RFC3339 timestamp")
	}
	var slotValue string
	if name == "replay-regional" {
		command.StringVar(&slotValue, "staffed-slots", "", "optional authored staffing assumption (0–128)")
	}
	if err := command.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *path == "" || *digest == "" || command.NArg() != 0 {
		return fmt.Errorf("provide --input and --sha256 for %s", name)
	}
	var cutoff time.Time
	if name != "replay-port" {
		var err error
		cutoff, err = time.Parse(time.RFC3339Nano, knownAt)
		if err != nil {
			return fmt.Errorf("provide --known-at with an RFC3339 timestamp")
		}
	}
	file, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (3<<20)+1))
	if err != nil {
		return err
	}
	var result any
	switch name {
	case "replay-supporting":
		result, err = geoenv.ReadSupportingSources(raw, cutoff, *digest)
	case "replay-port":
		result, err = geoenv.ReadPortReview(raw, *digest, port, baselineFrom, studyFrom, studyTo, minimum, consecutive)
	case "replay-aerial":
		result, err = geoenv.ReadAerialReview(raw, cutoff, *digest)
	case "replay-regional":
		var slots *int
		if slotValue != "" {
			count, err := strconv.Atoi(slotValue)
			if err != nil {
				return fmt.Errorf("staffed-slots must be an integer")
			}
			slots = &count
		}
		result, err = geoenv.ReadRegionalResponse(raw, cutoff, *digest, manifest, slots)
	default:
		result, err = geoenv.ReadCivilianAccessReplay(raw, cutoff, *digest, manifest)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
