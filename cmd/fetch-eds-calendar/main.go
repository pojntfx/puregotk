package main

import (
	"fmt"

	"github.com/jwijenbergh/puregotk/v4/ecal"
	"github.com/jwijenbergh/puregotk/v4/edataserver"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/icalglib"
)

func main() {
	registry, err := edataserver.NewSourceRegistrySync(nil)
	if err != nil {
		panic(err)
	}

	sources := registry.ListSources(edataserver.SOURCE_EXTENSION_CALENDAR)

	count := 0
	for s := sources; s != nil; s = s.Next {
		count++
	}
	fmt.Printf("Found %d calendar(s):\n\n", count)

	for s := sources; s != nil; s = s.Next {
		source := edataserver.SourceNewFromInternalPtr(s.Data)
		name := source.GetDisplayName()
		uid := source.GetUid()

		fmt.Printf("Calendar: %s (%s)\n", name, uid)

		client, err := ecal.ClientConnectSync(source, ecal.ECalClientSourceTypeEventsValue, 1, nil)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		calClient := ecal.ClientNewFromInternalPtr(client.GoPointer())

		var components *glib.SList
		ok, err := calClient.GetObjectListSync("#t", &components, nil)
		if err != nil || !ok {
			fmt.Printf("  Error getting objects: %v\n\n", err)
			continue
		}

		compCount := 0
		for c := components; c != nil; c = c.Next {
			compCount++
		}
		fmt.Printf("  Events: %d\n\n", compCount)

		for c := components; c != nil; c = c.Next {
			comp := icalglib.ComponentNewFromInternalPtr(c.Data)

			summary := comp.GetSummary()
			if summary == "" {
				summary = "(No title)"
			}
			description := comp.GetDescription()
			location := comp.GetLocation()
			dtstart := comp.GetDtstart()
			dtend := comp.GetDtend()

			fmt.Printf("  - %s\n", summary)
			if dtstart != nil {
				fmt.Printf("      Start: %s\n", dtstart.AsIcalString())
			}
			if dtend != nil {
				fmt.Printf("      End: %s\n", dtend.AsIcalString())
			}
			if location != "" {
				fmt.Printf("      Location: %s\n", location)
			}
			if description != "" {
				if len(description) > 100 {
					description = description[:100] + "..."
				}
				fmt.Printf("      Description: %s\n", description)
			}
			fmt.Println()
		}
	}
}
