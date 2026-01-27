package main

import (
	"fmt"

	"github.com/jwijenbergh/puregotk/v4/ecal"
	"github.com/jwijenbergh/puregotk/v4/edataserver"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/icalglib"
)

var statusMap = map[icalglib.PropertyStatus]string{
	icalglib.ICalStatusXValue:           "Custom",
	icalglib.ICalStatusTentativeValue:   "Tentative",
	icalglib.ICalStatusConfirmedValue:   "Confirmed",
	icalglib.ICalStatusCompletedValue:   "Completed",
	icalglib.ICalStatusNeedsactionValue: "Needs Action",
	icalglib.ICalStatusCancelledValue:   "Cancelled",
	icalglib.ICalStatusInprocessValue:   "In Process",
	icalglib.ICalStatusDraftValue:       "Draft",
	icalglib.ICalStatusFinalValue:       "Final",
	icalglib.ICalStatusSubmittedValue:   "Submitted",
	icalglib.ICalStatusPendingValue:     "Pending",
	icalglib.ICalStatusFailedValue:      "Failed",
	icalglib.ICalStatusDeletedValue:     "Deleted",
	icalglib.ICalStatusNoneValue:        "None",
}

func main() {
	registry, err := edataserver.NewSourceRegistrySync(nil)
	if err != nil {
		panic(err)
	}

	sources := registry.ListSources(edataserver.SOURCE_EXTENSION_TASK_LIST)

	count := 0
	for s := sources; s != nil; s = s.Next {
		count++
	}
	fmt.Printf("Found %d task list(s):\n\n", count)

	for s := sources; s != nil; s = s.Next {
		source := edataserver.SourceNewFromInternalPtr(s.Data)
		name := source.GetDisplayName()
		uid := source.GetUid()

		fmt.Printf("Task List: %s (%s)\n", name, uid)

		client, err := ecal.ClientConnectSync(source, ecal.ECalClientSourceTypeTasksValue, 1, nil)
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
		fmt.Printf("  Tasks: %d\n\n", compCount)

		for c := components; c != nil; c = c.Next {
			comp := icalglib.ComponentNewFromInternalPtr(c.Data)

			summary := comp.GetSummary()
			if summary == "" {
				summary = "(No title)"
			}
			description := comp.GetDescription()
			due := comp.GetDue()

			statusProp := comp.GetFirstProperty(icalglib.ICalStatusPropertyValue)
			status := "Not set"
			if statusProp != nil {
				s := comp.GetStatus()
				if name, ok := statusMap[s]; ok {
					status = name
				} else {
					status = "Unknown"
				}
			}

			priorityProp := comp.GetFirstProperty(icalglib.ICalPriorityPropertyValue)
			var priority int
			if priorityProp != nil {
				priority = priorityProp.GetPriority()
			}

			fmt.Printf("  - %s\n", summary)
			fmt.Printf("      Status: %s\n", status)
			if priority > 0 {
				fmt.Printf("      Priority: %d\n", priority)
			}
			if due != nil && !due.IsNullTime() {
				fmt.Printf("      Due: %s\n", due.AsIcalString())
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
