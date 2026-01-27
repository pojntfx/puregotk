package main

import (
	"fmt"

	"github.com/jwijenbergh/puregotk/v4/ebook"
	"github.com/jwijenbergh/puregotk/v4/ebookcontacts"
	"github.com/jwijenbergh/puregotk/v4/edataserver"
	"github.com/jwijenbergh/puregotk/v4/glib"
)

func main() {
	registry, err := edataserver.NewSourceRegistrySync(nil)
	if err != nil {
		panic(err)
	}

	sources := registry.ListSources(edataserver.SOURCE_EXTENSION_ADDRESS_BOOK)

	count := 0
	for s := sources; s != nil; s = s.Next {
		count++
	}
	fmt.Printf("Found %d address book(s):\n\n", count)

	for s := sources; s != nil; s = s.Next {
		source := edataserver.SourceNewFromInternalPtr(s.Data)
		name := source.GetDisplayName()
		uid := source.GetUid()

		fmt.Printf("Address Book: %s (%s)\n", name, uid)

		client, err := ebook.BookClientConnectSync(source, 1, nil)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		var contacts *glib.SList
		ok, err := client.GetContactsSync("", &contacts, nil)
		if err != nil || !ok {
			fmt.Printf("  Error getting contacts: %v\n\n", err)
			continue
		}

		contactCount := 0
		for c := contacts; c != nil; c = c.Next {
			contactCount++
		}
		fmt.Printf("  Contacts: %d\n\n", contactCount)

		for c := contacts; c != nil; c = c.Next {
			contact := ebookcontacts.ContactNewFromInternalPtr(c.Data)

			fullName := contact.GetPropertyFullName()
			givenName := contact.GetPropertyGivenName()
			familyName := contact.GetPropertyFamilyName()

			displayName := fullName
			if displayName == "" {
				displayName = givenName
				if familyName != "" {
					if displayName != "" {
						displayName += " "
					}
					displayName += familyName
				}
			}
			if displayName == "" {
				displayName = "(No name)"
			}

			fmt.Printf("  - %s\n", displayName)

			if nickname := contact.GetPropertyNickname(); nickname != "" {
				fmt.Printf("      Nickname: %s\n", nickname)
			}
			if email := contact.GetPropertyEmail1(); email != "" {
				fmt.Printf("      Email: %s\n", email)
			}
			if email2 := contact.GetPropertyEmail2(); email2 != "" {
				fmt.Printf("      Email 2: %s\n", email2)
			}
			if mobile := contact.GetPropertyMobilePhone(); mobile != "" {
				fmt.Printf("      Mobile: %s\n", mobile)
			}
			if home := contact.GetPropertyHomePhone(); home != "" {
				fmt.Printf("      Home: %s\n", home)
			}
			if work := contact.GetPropertyBusinessPhone(); work != "" {
				fmt.Printf("      Work: %s\n", work)
			}
			if org := contact.GetPropertyOrg(); org != "" {
				fmt.Printf("      Organization: %s\n", org)
			}
			if title := contact.GetPropertyTitle(); title != "" {
				fmt.Printf("      Title: %s\n", title)
			}
			fmt.Println()
		}
	}
}
