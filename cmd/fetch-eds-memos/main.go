package main

import (
	"fmt"
	"strings"

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

	sources := registry.ListSources(edataserver.SOURCE_EXTENSION_MEMO_LIST)

	count := 0
	for s := sources; s != nil; s = s.Next {
		count++
	}
	fmt.Printf("Found %d memo list(s):\n\n", count)

	for s := sources; s != nil; s = s.Next {
		source := edataserver.SourceNewFromInternalPtr(s.Data)
		name := source.GetDisplayName()
		uid := source.GetUid()

		fmt.Printf("Memo List: %s (%s)\n", name, uid)

		client, err := ecal.ClientConnectSync(source, ecal.ECalClientSourceTypeMemosValue, 1, nil)
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
		fmt.Printf("  Memos: %d\n\n", compCount)

		for c := components; c != nil; c = c.Next {
			comp := icalglib.ComponentNewFromInternalPtr(c.Data)

			summary := comp.GetSummary()
			if summary == "" {
				summary = "(No title)"
			}
			description := comp.GetDescription()
			dtstart := comp.GetDtstart()

			fmt.Printf("  - %s\n", summary)
			if dtstart != nil && !dtstart.IsNullTime() {
				fmt.Printf("      Date: %s\n", dtstart.AsIcalString())
			}

			if description != "" {
				lines := strings.Split(description, "\n")
				fmt.Printf("      Content:\n")
				maxLines := 5
				if len(lines) < maxLines {
					maxLines = len(lines)
				}
				for i := 0; i < maxLines; i++ {
					fmt.Printf("        %s\n", lines[i])
				}
				if len(lines) > 5 {
					fmt.Printf("        ... (%d more lines)\n", len(lines)-5)
				}
			}
			fmt.Println()
		}
	}
}
